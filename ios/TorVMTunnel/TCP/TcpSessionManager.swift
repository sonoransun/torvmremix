import Foundation

/// Manages all active ``TcpSession`` instances.
///
/// Incoming TCP packets are routed to the appropriate session based on their
/// 4-tuple ``SessionKey``. New SYN packets create a fresh session; packets for
/// unknown sessions receive a RST.
///
/// A background cleanup task periodically reaps sessions that are CLOSED,
/// stuck in TIME_WAIT beyond 60 seconds, or idle for longer than 5 minutes.
actor TcpSessionManager {
    private let socksHost: String
    private let socksPort: UInt16
    private let packetWriter: (Data) -> Void

    /// Optional isolation key provider. When set, maps a session key
    /// to a SOCKS5 username/password pair for per-app circuit isolation.
    /// Tor creates separate circuits for different credential combos.
    private let isolationKeyProvider: ((SessionKey) -> (String, String)?)?

    private var sessions: [SessionKey: TcpSession] = [:]
    private var cleanupTask: Task<Void, Never>?

    /// Maximum number of concurrent sessions.
    private static let maxSessions = 1024

    /// Interval between cleanup sweeps.
    private static let cleanupIntervalSeconds: TimeInterval = 30

    /// TIME_WAIT sessions are removed after this duration.
    private static let timeWaitTimeout: TimeInterval = 60

    /// Sessions with no activity for this long are forcefully closed.
    private static let idleTimeout: TimeInterval = 300  // 5 minutes

    /// SYN_RECEIVED sessions are reaped after this duration.
    private static let synReceivedTimeout: TimeInterval = 10

    /// Create a new TCP session manager.
    ///
    /// - Parameters:
    ///   - socksHost: SOCKS5 proxy hostname
    ///   - socksPort: SOCKS5 proxy port
    ///   - packetWriter: callback to write packets back to the TUN device
    ///   - isolationKeyProvider: optional callback for per-app circuit isolation
    init(
        socksHost: String,
        socksPort: UInt16,
        packetWriter: @escaping (Data) -> Void,
        isolationKeyProvider: ((SessionKey) -> (String, String)?)? = nil
    ) {
        self.socksHost = socksHost
        self.socksPort = socksPort
        self.packetWriter = packetWriter
        self.isolationKeyProvider = isolationKeyProvider
    }

    /// Start the periodic session cleanup task.
    func start() {
        cleanupTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(nanoseconds: UInt64(Self.cleanupIntervalSeconds * 1_000_000_000))
                guard !Task.isCancelled else { break }
                await self?.cleanup()
            }
        }
    }

    /// Stop all sessions and cancel the cleanup task.
    func stop() {
        cleanupTask?.cancel()
        cleanupTask = nil
        for session in sessions.values {
            Task { await session.close() }
        }
        sessions.removeAll()
    }

    /// Route a TCP packet to the correct session.
    ///
    /// - SYN (without ACK) creates a new session.
    /// - All other packets are forwarded to the existing session.
    /// - Packets for unknown sessions trigger a RST reply.
    func handlePacket(
        ipHeader: IPv4Header,
        tcpHeader: TcpHeader,
        payload: Data
    ) async {
        let key = SessionKey(
            srcAddr: ipHeader.sourceAddressBytes,
            srcPort: tcpHeader.sourcePort,
            dstAddr: ipHeader.destAddressBytes,
            dstPort: tcpHeader.destPort
        )

        if tcpHeader.isSyn && !tcpHeader.isAck {
            // Enforce session limit to prevent SYN flood exhaustion
            if sessions.count >= Self.maxSessions {
                sendRst(ipHeader: ipHeader, tcpHeader: tcpHeader)
                return
            }

            // New connection -- remove any stale session with the same key
            if let old = sessions.removeValue(forKey: key) {
                await old.close()
            }

            let isolation = isolationKeyProvider?(key)
            let session = TcpSession(
                key: key,
                socksHost: socksHost,
                socksPort: socksPort,
                packetWriter: packetWriter,
                isolationUsername: isolation?.0,
                isolationPassword: isolation?.1
            )
            sessions[key] = session
            await session.handlePacket(ipHeader: ipHeader, tcpHeader: tcpHeader, payload: payload)
        } else {
            if let session = sessions[key] {
                await session.handlePacket(ipHeader: ipHeader, tcpHeader: tcpHeader, payload: payload)
                let closed = await session.isClosed
                if closed {
                    sessions.removeValue(forKey: key)
                }
            } else {
                // No session found -- send RST so the client stops retrying
                sendRst(ipHeader: ipHeader, tcpHeader: tcpHeader)
            }
        }
    }

    /// Send a RST packet in response to a packet for an unknown session.
    private func sendRst(ipHeader: IPv4Header, tcpHeader: TcpHeader) {
        let ackNum: UInt32
        if tcpHeader.isSyn {
            ackNum = UInt32(truncatingIfNeeded: tcpHeader.sequenceNumber) &+ 1
        } else {
            ackNum = UInt32(truncatingIfNeeded: tcpHeader.sequenceNumber)
        }

        let packet = PacketBuilder.buildTcpPacket(
            srcAddr: ipHeader.destAddressBytes,
            dstAddr: ipHeader.sourceAddressBytes,
            srcPort: tcpHeader.destPort,
            dstPort: tcpHeader.sourcePort,
            seqNum: UInt32(truncatingIfNeeded: tcpHeader.ackNumber),
            ackNum: ackNum,
            flags: TcpHeader.flagRST | TcpHeader.flagACK,
            window: 0,
            payload: Data()
        )
        packetWriter(packet)
    }

    /// Remove sessions that are CLOSED, timed-out in TIME_WAIT, or idle too long.
    private func cleanup() async {
        let now = Date()
        var keysToRemove: [SessionKey] = []

        for (key, session) in sessions {
            let lastActivity = await session.lastActivity
            let elapsed = now.timeIntervalSince(lastActivity)
            let sessionState = await session.state

            let shouldRemove: Bool
            switch sessionState {
            case .closed:
                shouldRemove = true
            case .timeWait:
                shouldRemove = elapsed > Self.timeWaitTimeout
            case .synReceived:
                shouldRemove = elapsed > Self.synReceivedTimeout
            default:
                shouldRemove = elapsed > Self.idleTimeout
            }

            if shouldRemove {
                await session.close()
                keysToRemove.append(key)
            }
        }

        for key in keysToRemove {
            sessions.removeValue(forKey: key)
        }
    }
}
