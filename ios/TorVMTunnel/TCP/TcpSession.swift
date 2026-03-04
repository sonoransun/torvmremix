import Foundation
import Network

/// Manages the state of a single proxied TCP connection.
///
/// For each connection from an iOS app the session:
/// 1. Accepts the SYN and replies with SYN+ACK.
/// 2. Connects to the real destination via SOCKS5 (through Tor).
/// 3. Relays data bidirectionally until the connection closes.
///
/// All response packets are written back to the TUN device through the `packetWriter` callback.
/// Uses Swift `actor` for thread safety instead of Kotlin Mutex.
actor TcpSession {
    /// Maximum segment size for data sent back to the client.
    static let mss = 1400

    /// Maximum advertised receive window.
    static let windowSize = 65535

    let key: SessionKey
    private let socksHost: String
    private let socksPort: UInt16
    private let packetWriter: (Data) -> Void
    private let isolationUsername: String?
    private let isolationPassword: String?

    private(set) var state: TcpState = .listen

    private var clientIsn: UInt32 = 0
    private var ourIsn: UInt32 = UInt32.random(in: 0...UInt32.max)
    private var clientSeq: UInt32 = 0   // next expected sequence number from the client
    private var ourSeq: UInt32 = 0      // next sequence number we will send

    private var upstreamConnection: NWConnection?
    private var upstreamTask: Task<Void, Never>?
    private var upstreamConnected: Bool = false

    private(set) var lastActivity: Date = Date()

    var isClosed: Bool {
        return state == .closed
    }

    /// Congestion controller for this session.
    var congestion = CongestionController()

    /// Create a new TCP session.
    ///
    /// - Parameters:
    ///   - key: the 4-tuple session key
    ///   - socksHost: SOCKS5 proxy hostname
    ///   - socksPort: SOCKS5 proxy port
    ///   - packetWriter: callback to write packets back to the TUN device
    ///   - isolationUsername: optional SOCKS5 username for circuit isolation
    ///   - isolationPassword: optional SOCKS5 password for circuit isolation
    init(
        key: SessionKey,
        socksHost: String,
        socksPort: UInt16,
        packetWriter: @escaping (Data) -> Void,
        isolationUsername: String? = nil,
        isolationPassword: String? = nil
    ) {
        self.key = key
        self.socksHost = socksHost
        self.socksPort = socksPort
        self.packetWriter = packetWriter
        self.isolationUsername = isolationUsername
        self.isolationPassword = isolationPassword
        self.ourSeq = ourIsn &+ 1
    }

    // MARK: - Packet Dispatch

    /// Process an incoming TCP packet for this session.
    ///
    /// Dispatches to the appropriate handler based on current state and the
    /// TCP flags present in the header.
    func handlePacket(
        ipHeader: IPv4Header,
        tcpHeader: TcpHeader,
        payload: Data
    ) async {
        lastActivity = Date()

        switch state {
        case .listen:
            if tcpHeader.isSyn && !tcpHeader.isAck {
                await handleSyn(tcpHeader: tcpHeader)
            }

        case .synReceived:
            if tcpHeader.isAck && !tcpHeader.isSyn {
                // Handshake ACK from client
                if upstreamConnected {
                    state = .established
                }
                // If upstream is not connected yet the session stays in
                // synReceived; once connectUpstream() finishes it will
                // set upstreamConnected = true and the next ACK (or data
                // packet) will transition the state.
            } else if tcpHeader.isRst {
                close()
            }

        case .established:
            if tcpHeader.isRst {
                close()
            } else if tcpHeader.isFin {
                await handleFin(tcpHeader: tcpHeader)
            } else if !payload.isEmpty {
                await handleEstablishedData(tcpHeader: tcpHeader, payload: payload)
            } else if tcpHeader.isAck {
                // Pure ACK -- update congestion controller
                congestion.onAck(0)
            }

        case .closeWait:
            // Waiting for our side to finish; ignore further data
            if tcpHeader.isRst { close() }

        case .lastAck:
            if tcpHeader.isAck {
                state = .closed
            } else if tcpHeader.isRst {
                close()
            }

        case .finWait1:
            if tcpHeader.isFin && tcpHeader.isAck {
                // Simultaneous close
                clientSeq = UInt32(truncatingIfNeeded: tcpHeader.sequenceNumber) &+ 1
                sendTcpToTun(flags: TcpHeader.flagACK)
                state = .timeWait
            } else if tcpHeader.isAck {
                state = .finWait2
            } else if tcpHeader.isFin {
                clientSeq = UInt32(truncatingIfNeeded: tcpHeader.sequenceNumber) &+ 1
                sendTcpToTun(flags: TcpHeader.flagACK)
                state = .timeWait
            }

        case .finWait2:
            if tcpHeader.isFin {
                clientSeq = UInt32(truncatingIfNeeded: tcpHeader.sequenceNumber) &+ 1
                sendTcpToTun(flags: TcpHeader.flagACK)
                state = .timeWait
            }

        case .timeWait, .closed:
            // Ignore; session will be reaped by cleanup
            break
        }
    }

    // MARK: - SYN Handling

    private func handleSyn(tcpHeader: TcpHeader) async {
        clientIsn = UInt32(truncatingIfNeeded: tcpHeader.sequenceNumber)
        clientSeq = clientIsn &+ 1

        // Send SYN+ACK
        sendTcpToTun(
            flags: TcpHeader.flagSYN | TcpHeader.flagACK,
            seqOverride: ourIsn
        )

        state = .synReceived

        // Begin upstream SOCKS5 connection asynchronously
        await connectUpstream()
    }

    // MARK: - Data Relay (client -> upstream)

    private func handleEstablishedData(
        tcpHeader: TcpHeader,
        payload: Data
    ) async {
        // If we were still in synReceived but upstream finished connecting,
        // promote to established now.
        if state == .synReceived && upstreamConnected {
            state = .established
        }

        guard let connection = upstreamConnection else { return }

        do {
            try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
                connection.send(content: payload, completion: .contentProcessed { error in
                    if let error = error {
                        continuation.resume(throwing: error)
                    } else {
                        continuation.resume()
                    }
                })
            }
        } catch {
            close()
            return
        }

        clientSeq = clientSeq &+ UInt32(payload.count)
        sendTcpToTun(flags: TcpHeader.flagACK)
    }

    // MARK: - FIN Handling

    private func handleFin(tcpHeader: TcpHeader) async {
        // ACK the client's FIN
        clientSeq = UInt32(truncatingIfNeeded: tcpHeader.sequenceNumber) &+ 1
        sendTcpToTun(flags: TcpHeader.flagACK)
        state = .closeWait

        // Close upstream write direction
        upstreamConnection?.send(content: nil, contentContext: .finalMessage, isComplete: true,
                                 completion: .contentProcessed { _ in })

        // Send our own FIN
        sendTcpToTun(flags: TcpHeader.flagFIN | TcpHeader.flagACK)
        ourSeq = ourSeq &+ 1
        state = .lastAck
    }

    // MARK: - Packet Injection into TUN

    /// Build a TCP packet heading back to the client and write it to the TUN device.
    ///
    /// Source and destination are swapped relative to the original client
    /// packet so that the packet travels FROM the remote server TO the app.
    private func sendTcpToTun(
        flags: UInt16,
        payload: Data = Data(),
        seqOverride: UInt32? = nil
    ) {
        let packet = PacketBuilder.buildTcpPacket(
            srcAddr: key.dstAddr,
            dstAddr: key.srcAddr,
            srcPort: key.dstPort,
            dstPort: key.srcPort,
            seqNum: seqOverride ?? ourSeq,
            ackNum: clientSeq,
            flags: flags,
            window: UInt16(clamping: congestion.getWindowSize()),
            payload: payload
        )
        if !payload.isEmpty {
            congestion.onSend(payload.count)
        }
        packetWriter(packet)
    }

    // MARK: - Upstream SOCKS5 Connection

    private func connectUpstream() async {
        do {
            let client = Socks5Client(proxyHost: socksHost, proxyPort: socksPort)
            client.authUsername = isolationUsername
            client.authPassword = isolationPassword
            let connection = try await client.connect(destAddr: key.dstAddr, destPort: key.dstPort)

            upstreamConnection = connection
            upstreamConnected = true

            // If the three-way handshake ACK already arrived while we were
            // connecting, promote to established now.
            if state == .synReceived {
                state = .established
            }

            startUpstreamReader(connection: connection)
        } catch {
            // SOCKS5 connect failed -- send RST to client
            sendTcpToTun(flags: TcpHeader.flagRST | TcpHeader.flagACK)
            state = .closed
        }
    }

    // MARK: - Upstream Reader (upstream -> client)

    private func startUpstreamReader(connection: NWConnection) {
        upstreamTask = Task { [weak self] in
            while !Task.isCancelled {
                do {
                    let data = try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Data, Error>) in
                        connection.receive(minimumIncompleteLength: 1, maximumLength: TcpSession.mss) { content, _, isComplete, error in
                            if let error = error {
                                continuation.resume(throwing: error)
                            } else if let data = content, !data.isEmpty {
                                continuation.resume(returning: data)
                            } else if isComplete {
                                continuation.resume(throwing: UpstreamEOF())
                            } else {
                                continuation.resume(throwing: UpstreamEOF())
                            }
                        }
                    }

                    guard let self = self else { return }
                    await self.handleUpstreamData(data)
                } catch {
                    break
                }
            }

            // Upstream EOF -- send FIN to client
            guard let self = self else { return }
            await self.handleUpstreamEOF()
        }
    }

    private func handleUpstreamData(_ data: Data) {
        sendTcpToTun(
            flags: TcpHeader.flagPSH | TcpHeader.flagACK,
            payload: data
        )
        ourSeq = ourSeq &+ UInt32(data.count)
        lastActivity = Date()
    }

    private func handleUpstreamEOF() {
        if state == .established {
            sendTcpToTun(flags: TcpHeader.flagFIN | TcpHeader.flagACK)
            ourSeq = ourSeq &+ 1
            state = .finWait1
        }
    }

    // MARK: - Cleanup

    /// Forcefully close the session, releasing all resources.
    func close() {
        upstreamTask?.cancel()
        upstreamConnection?.cancel()
        upstreamConnection = nil
        state = .closed
    }
}

/// Sentinel error used to signal upstream EOF in the reader loop.
private struct UpstreamEOF: Error {}
