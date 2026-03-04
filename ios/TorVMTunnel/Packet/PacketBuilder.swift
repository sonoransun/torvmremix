import Foundation

/// Utility for assembling complete IPv4 packets with proper checksums.
///
/// Builds TCP or UDP packets by:
/// 1. Creating the transport header
/// 2. Creating the IP header with correct total length
/// 3. Computing the transport-layer checksum (using pseudo-header)
/// 4. Computing the IP header checksum
/// 5. Concatenating all parts into a single Data
enum PacketBuilder {

    private static let defaultTTL: UInt8 = 64
    private static let ipHeaderSize = 20
    static let windowSize: UInt16 = 65535

    /// Build a complete IPv4/TCP packet.
    ///
    /// - Parameters:
    ///   - srcAddr: source IPv4 address (4 bytes)
    ///   - dstAddr: destination IPv4 address (4 bytes)
    ///   - srcPort: source TCP port
    ///   - dstPort: destination TCP port
    ///   - seqNum: TCP sequence number
    ///   - ackNum: TCP acknowledgment number
    ///   - flags: TCP flags (e.g., TcpHeader.flagSYN | TcpHeader.flagACK)
    ///   - window: TCP window size
    ///   - payload: TCP payload data
    ///   - identification: IP identification field
    /// - Returns: the complete IP packet as Data
    static func buildTcpPacket(
        srcAddr: Data,
        dstAddr: Data,
        srcPort: UInt16,
        dstPort: UInt16,
        seqNum: UInt32,
        ackNum: UInt32,
        flags: UInt16,
        window: UInt16,
        payload: Data = Data(),
        identification: UInt16 = 0
    ) -> Data {
        let tcpHeaderSize = 20 // No options
        let totalLength = ipHeaderSize + tcpHeaderSize + payload.count

        // Build TCP header with checksum = 0 (will be computed below)
        let tcpHeader = TcpHeader(
            sourcePort: srcPort,
            destPort: dstPort,
            sequenceNumber: seqNum,
            ackNumber: ackNum,
            dataOffset: 5, // 20 bytes / 4
            reserved: 0,
            flags: flags,
            window: window,
            checksum: 0,
            urgentPointer: 0
        )

        // Build IP header with checksum = 0 (will be recalculated by toBytes())
        let ipHeader = IPv4Header(
            version: 4,
            ihl: 5,
            tos: 0,
            totalLength: UInt16(totalLength),
            identification: identification,
            flags: 0x02, // Don't Fragment
            fragmentOffset: 0,
            ttl: defaultTTL,
            protocol: IPv4Header.protocolTCP,
            headerChecksum: 0,
            sourceAddress: Data(srcAddr),
            destAddress: Data(dstAddr)
        )

        // Compute TCP checksum using pseudo-header
        var tcpBytes = tcpHeader.toBytes()
        let tcpChecksum = Checksum.tcpUdpChecksum(ipHeader: ipHeader, transportHeader: tcpBytes, payload: payload)

        // Update TCP header bytes with the computed checksum
        tcpBytes[16] = UInt8(tcpChecksum >> 8)
        tcpBytes[17] = UInt8(tcpChecksum & 0xFF)

        // Serialize IP header (recalculates IP checksum internally)
        let ipBytes = ipHeader.toBytes()

        // Assemble the complete packet
        var packet = Data(count: totalLength)
        packet.replaceSubrange(0..<ipHeaderSize, with: ipBytes)
        packet.replaceSubrange(ipHeaderSize..<(ipHeaderSize + tcpHeaderSize), with: tcpBytes)
        if !payload.isEmpty {
            packet.replaceSubrange((ipHeaderSize + tcpHeaderSize)..<totalLength, with: payload)
        }

        return packet
    }

    /// Build a complete IPv4/UDP packet.
    ///
    /// - Parameters:
    ///   - srcAddr: source IPv4 address (4 bytes)
    ///   - dstAddr: destination IPv4 address (4 bytes)
    ///   - srcPort: source UDP port
    ///   - dstPort: destination UDP port
    ///   - payload: UDP payload data
    /// - Returns: the complete IP packet as Data
    static func buildUdpPacket(
        srcAddr: Data,
        dstAddr: Data,
        srcPort: UInt16,
        dstPort: UInt16,
        payload: Data
    ) -> Data {
        let udpLength = UdpHeader.headerSize + payload.count
        let totalLength = ipHeaderSize + udpLength

        // Build UDP header with checksum = 0 (will be computed below)
        let udpHeader = UdpHeader(
            sourcePort: srcPort,
            destPort: dstPort,
            length: UInt16(udpLength),
            checksum: 0
        )

        // Build IP header with checksum = 0 (will be recalculated by toBytes())
        let ipHeader = IPv4Header(
            version: 4,
            ihl: 5,
            tos: 0,
            totalLength: UInt16(totalLength),
            identification: 0,
            flags: 0x02, // Don't Fragment
            fragmentOffset: 0,
            ttl: defaultTTL,
            protocol: IPv4Header.protocolUDP,
            headerChecksum: 0,
            sourceAddress: Data(srcAddr),
            destAddress: Data(dstAddr)
        )

        // Compute UDP checksum using pseudo-header
        var udpBytes = udpHeader.toBytes()
        let udpChecksum = Checksum.tcpUdpChecksum(ipHeader: ipHeader, transportHeader: udpBytes, payload: payload)

        // Update UDP header bytes with the computed checksum
        udpBytes[6] = UInt8(udpChecksum >> 8)
        udpBytes[7] = UInt8(udpChecksum & 0xFF)

        // Serialize IP header (recalculates IP checksum internally)
        let ipBytes = ipHeader.toBytes()

        // Assemble the complete packet
        var packet = Data(count: totalLength)
        packet.replaceSubrange(0..<ipHeaderSize, with: ipBytes)
        packet.replaceSubrange(ipHeaderSize..<(ipHeaderSize + UdpHeader.headerSize), with: udpBytes)
        if !payload.isEmpty {
            packet.replaceSubrange((ipHeaderSize + UdpHeader.headerSize)..<totalLength, with: payload)
        }

        return packet
    }
}
