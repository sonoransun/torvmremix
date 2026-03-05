import Foundation
import Network
import NetworkExtension
import os

/// Relays DNS queries to an external DNS resolver (typically Tor's DNSPort)
/// and injects the responses back into the TUN device.
///
/// Each query is forwarded as a plain UDP datagram via NWConnection.
/// On iOS, routing loop prevention is handled by NEIPv4Settings.excludedRoutes
/// rather than Android's socket protection pattern.
final class DnsRelay {

    private let dnsHost: String
    private let dnsPort: UInt16
    private let packetFlow: NEPacketTunnelFlow

    private static let dnsBufferSize = 4096
    private static let dnsTimeoutSeconds: TimeInterval = 5
    private static let maxConcurrentQueries = 32

    private let activeLock = os.OSAllocatedUnfairLock(initialState: Int(0))
    private let queue = DispatchQueue(label: "com.torvm.dns", attributes: .concurrent)
    private var isStopped = false

    init(dnsHost: String, dnsPort: UInt16, packetFlow: NEPacketTunnelFlow) {
        self.dnsHost = dnsHost
        self.dnsPort = dnsPort
        self.packetFlow = packetFlow
    }

    func start() {
        isStopped = false
    }

    func stop() {
        isStopped = true
    }

    /// Handle an intercepted DNS query packet.
    func handlePacket(ipHeader: IPv4Header, udpHeader: UdpHeader, payload: Data) {
        guard !isStopped else { return }

        let allowed = activeLock.withLock { (count: inout Int) -> Bool in
            guard count < Self.maxConcurrentQueries else { return false }
            count += 1
            return true
        }
        guard allowed else { return }

        queue.async { [weak self] in
            defer {
                self?.activeLock.withLock { (count: inout Int) in
                    count -= 1
                }
            }
            guard let self = self, !self.isStopped else { return }

            do {
                let response = try self.forwardQuerySync(payload)

                let responsePacket = PacketBuilder.buildUdpPacket(
                    srcAddr: ipHeader.destAddress,
                    dstAddr: ipHeader.sourceAddress,
                    srcPort: udpHeader.destPort,
                    dstPort: udpHeader.sourcePort,
                    payload: response
                )

                self.packetFlow.writePackets(
                    [responsePacket],
                    withProtocols: [NSNumber(value: AF_INET)]
                )
            } catch {
                // Query failed — drop silently.
            }
        }
    }

    /// Forward a raw DNS query to the upstream resolver using NWConnection (UDP).
    /// TODO: Replace synchronous semaphore-based NWConnection pattern with async/await
    /// using withCheckedThrowingContinuation for better thread utilization.
    private func forwardQuerySync(_ query: Data) throws -> Data {
        guard query.count >= 2 else {
            throw DnsRelayError.queryTooShort
        }

        let sem = DispatchSemaphore(value: 0)
        var result: Result<Data, Error>?

        let endpoint = NWEndpoint.hostPort(
            host: NWEndpoint.Host(dnsHost),
            port: NWEndpoint.Port(rawValue: dnsPort)!
        )
        let params = NWParameters.udp
        let connection = NWConnection(to: endpoint, using: params)

        connection.stateUpdateHandler = { state in
            switch state {
            case .ready:
                // Send query
                connection.send(content: query, completion: .contentProcessed { error in
                    if let error = error {
                        result = .failure(error)
                        sem.signal()
                        return
                    }
                    // Receive response
                    connection.receiveMessage { data, _, _, recvError in
                        defer { sem.signal() }
                        if let error = recvError {
                            result = .failure(error)
                            return
                        }
                        guard let data = data, data.count >= 2 else {
                            result = .failure(DnsRelayError.emptyResponse)
                            return
                        }
                        // Validate transaction ID
                        if data[0] != query[0] || data[1] != query[1] {
                            result = .failure(DnsRelayError.transactionIdMismatch)
                            return
                        }
                        result = .success(data)
                    }
                })
            case .failed(let error):
                result = .failure(error)
                sem.signal()
            case .cancelled:
                result = .failure(DnsRelayError.cancelled)
                sem.signal()
            default:
                break
            }
        }

        connection.start(queue: queue)

        let timeout = DispatchTime.now() + Self.dnsTimeoutSeconds
        if sem.wait(timeout: timeout) == .timedOut {
            connection.cancel()
            throw DnsRelayError.timeout
        }

        connection.cancel()

        switch result {
        case .success(let data):
            return data
        case .failure(let error):
            throw error
        case .none:
            throw DnsRelayError.emptyResponse
        }
    }

    enum DnsRelayError: LocalizedError {
        case queryTooShort
        case emptyResponse
        case transactionIdMismatch
        case timeout
        case cancelled

        var errorDescription: String? {
            switch self {
            case .queryTooShort: return "DNS query too short"
            case .emptyResponse: return "Empty DNS response"
            case .transactionIdMismatch: return "DNS transaction ID mismatch"
            case .timeout: return "DNS query timed out"
            case .cancelled: return "DNS query cancelled"
            }
        }
    }
}
