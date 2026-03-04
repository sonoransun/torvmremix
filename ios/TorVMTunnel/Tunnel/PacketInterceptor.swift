import Foundation

/// Classifies raw IP packets read from the TUN device and dispatches them
/// to the appropriate handler.
///
/// - TCP packets go to the TcpSessionManager for SOCKS5 proxying through Tor.
/// - UDP packets to port 53 go to the DnsRelay for resolution via Tor's DNSPort.
/// - All other UDP and non-IPv4 packets are silently dropped.
final class PacketInterceptor {

    private let tcpSessionManager: TcpSessionManager
    private let dnsRelay: DnsRelay

    private static let minIPHeaderSize = 20
    private static let minTCPHeaderSize = 20
    private static let udpHeaderSize = 8
    private static let dnsPort: UInt16 = 53

    init(tcpSessionManager: TcpSessionManager, dnsRelay: DnsRelay) {
        self.tcpSessionManager = tcpSessionManager
        self.dnsRelay = dnsRelay
    }

    /// Parse and dispatch a single IP packet.
    func intercept(packet: Data, length: Int) {
        guard length >= Self.minIPHeaderSize else { return }

        // Check IP version nibble
        let version = packet[0] >> 4
        guard version == 4 else { return }

        guard let ipHeader = try? IPv4Header.parse(data: packet) else { return }
        let transportOffset = ipHeader.headerLength

        switch ipHeader.`protocol` {
        case IPv4Header.protocolTCP:
            guard length >= transportOffset + Self.minTCPHeaderSize else { return }

            // Verify TCP checksum
            guard Checksum.verifyTransportChecksum(
                ipHeader: ipHeader,
                packet: packet,
                transportOffset: transportOffset,
                length: length
            ) else { return }

            guard let tcpHeader = try? TcpHeader.parse(data: packet, offset: transportOffset) else { return }

            let payloadOffset = transportOffset + tcpHeader.headerLength
            let payload = payloadOffset < length
                ? packet.subdata(in: payloadOffset..<length)
                : Data()

            Task {
                await tcpSessionManager.handlePacket(
                    ipHeader: ipHeader,
                    tcpHeader: tcpHeader,
                    payload: payload
                )
            }

        case IPv4Header.protocolUDP:
            guard length >= transportOffset + Self.udpHeaderSize else { return }

            guard let udpHeader = try? UdpHeader.parse(data: packet, offset: transportOffset) else { return }

            if udpHeader.destPort == Self.dnsPort {
                let payloadOffset = transportOffset + Self.udpHeaderSize
                guard payloadOffset < length else { return }
                let payload = packet.subdata(in: payloadOffset..<length)

                dnsRelay.handlePacket(
                    ipHeader: ipHeader,
                    udpHeader: udpHeader,
                    payload: payload
                )
            }
            // Non-DNS UDP: dropped (Tor does not support generic UDP)

        default:
            break  // ICMP, etc. dropped
        }
    }
}
