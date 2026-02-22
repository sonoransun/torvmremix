package com.torvm.android.packet

/**
 * Utility object for assembling complete IPv4 packets with proper checksums.
 *
 * Builds TCP or UDP packets by:
 * 1. Creating the transport header
 * 2. Creating the IP header with correct total length
 * 3. Computing the transport-layer checksum (using pseudo-header)
 * 4. Computing the IP header checksum
 * 5. Concatenating all parts into a single byte array
 */
object PacketBuilder {

    private const val DEFAULT_TTL = 64
    private const val IP_HEADER_SIZE = 20

    /**
     * Build a complete IPv4/TCP packet.
     *
     * @param srcAddr source IPv4 address (4 bytes)
     * @param dstAddr destination IPv4 address (4 bytes)
     * @param srcPort source TCP port
     * @param dstPort destination TCP port
     * @param seqNum TCP sequence number (unsigned 32-bit stored as Long)
     * @param ackNum TCP acknowledgment number (unsigned 32-bit stored as Long)
     * @param flags TCP flags (e.g., TcpHeader.FLAG_SYN or TcpHeader.FLAG_ACK)
     * @param window TCP window size
     * @param payload TCP payload data
     * @param identification IP identification field
     * @return the complete IP packet as a byte array
     */
    fun buildTcpPacket(
        srcAddr: ByteArray,
        dstAddr: ByteArray,
        srcPort: Int,
        dstPort: Int,
        seqNum: Long,
        ackNum: Long,
        flags: Int,
        window: Int,
        payload: ByteArray = byteArrayOf(),
        identification: Int = 0
    ): ByteArray {
        val tcpHeaderSize = 20 // No options
        val totalLength = IP_HEADER_SIZE + tcpHeaderSize + payload.size

        // Build TCP header with checksum = 0 (will be computed below)
        val tcpHeader = TcpHeader(
            sourcePort = srcPort,
            destPort = dstPort,
            sequenceNumber = seqNum,
            ackNumber = ackNum,
            dataOffset = 5, // 20 bytes / 4
            reserved = 0,
            flags = flags,
            window = window,
            checksum = 0,
            urgentPointer = 0
        )

        // Build IP header with checksum = 0 (will be recalculated by toBytes())
        val ipHeader = IPv4Header(
            version = 4,
            ihl = 5,
            tos = 0,
            totalLength = totalLength,
            identification = identification,
            flags = 0x02, // Don't Fragment
            fragmentOffset = 0,
            ttl = DEFAULT_TTL,
            protocol = IPv4Header.PROTOCOL_TCP,
            headerChecksum = 0,
            sourceAddress = srcAddr.copyOf(),
            destAddress = dstAddr.copyOf()
        )

        // Compute TCP checksum using pseudo-header
        val tcpBytes = tcpHeader.toBytes()
        val tcpChecksum = Checksum.tcpUdpChecksum(ipHeader, tcpBytes, payload)

        // Update TCP header bytes with the computed checksum
        tcpBytes[16] = (tcpChecksum shr 8).toByte()
        tcpBytes[17] = tcpChecksum.toByte()

        // Serialize IP header (recalculates IP checksum internally)
        val ipBytes = ipHeader.toBytes()

        // Assemble the complete packet
        val packet = ByteArray(totalLength)
        System.arraycopy(ipBytes, 0, packet, 0, IP_HEADER_SIZE)
        System.arraycopy(tcpBytes, 0, packet, IP_HEADER_SIZE, tcpHeaderSize)
        if (payload.isNotEmpty()) {
            System.arraycopy(payload, 0, packet, IP_HEADER_SIZE + tcpHeaderSize, payload.size)
        }

        return packet
    }

    /**
     * Build a complete IPv4/UDP packet.
     *
     * @param srcAddr source IPv4 address (4 bytes)
     * @param dstAddr destination IPv4 address (4 bytes)
     * @param srcPort source UDP port
     * @param dstPort destination UDP port
     * @param payload UDP payload data
     * @return the complete IP packet as a byte array
     */
    fun buildUdpPacket(
        srcAddr: ByteArray,
        dstAddr: ByteArray,
        srcPort: Int,
        dstPort: Int,
        payload: ByteArray
    ): ByteArray {
        val udpLength = UdpHeader.HEADER_SIZE + payload.size
        val totalLength = IP_HEADER_SIZE + udpLength

        // Build UDP header with checksum = 0 (will be computed below)
        val udpHeader = UdpHeader(
            sourcePort = srcPort,
            destPort = dstPort,
            length = udpLength,
            checksum = 0
        )

        // Build IP header with checksum = 0 (will be recalculated by toBytes())
        val ipHeader = IPv4Header(
            version = 4,
            ihl = 5,
            tos = 0,
            totalLength = totalLength,
            identification = 0,
            flags = 0x02, // Don't Fragment
            fragmentOffset = 0,
            ttl = DEFAULT_TTL,
            protocol = IPv4Header.PROTOCOL_UDP,
            headerChecksum = 0,
            sourceAddress = srcAddr.copyOf(),
            destAddress = dstAddr.copyOf()
        )

        // Compute UDP checksum using pseudo-header
        val udpBytes = udpHeader.toBytes()
        val udpChecksum = Checksum.tcpUdpChecksum(ipHeader, udpBytes, payload)

        // Update UDP header bytes with the computed checksum
        udpBytes[6] = (udpChecksum shr 8).toByte()
        udpBytes[7] = udpChecksum.toByte()

        // Serialize IP header (recalculates IP checksum internally)
        val ipBytes = ipHeader.toBytes()

        // Assemble the complete packet
        val packet = ByteArray(totalLength)
        System.arraycopy(ipBytes, 0, packet, 0, IP_HEADER_SIZE)
        System.arraycopy(udpBytes, 0, packet, IP_HEADER_SIZE, UdpHeader.HEADER_SIZE)
        if (payload.isNotEmpty()) {
            System.arraycopy(payload, 0, packet, IP_HEADER_SIZE + UdpHeader.HEADER_SIZE, payload.size)
        }

        return packet
    }
}
