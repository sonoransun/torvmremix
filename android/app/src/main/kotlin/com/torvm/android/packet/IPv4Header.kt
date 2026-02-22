package com.torvm.android.packet

/**
 * Parses and builds IPv4 headers (minimum 20 bytes).
 *
 * All multi-byte fields use network byte order (big-endian).
 * IP addresses are stored as ByteArray(4).
 */
data class IPv4Header(
    val version: Int,
    val ihl: Int,
    val tos: Int,
    val totalLength: Int,
    val identification: Int,
    val flags: Int,
    val fragmentOffset: Int,
    val ttl: Int,
    val protocol: Int,
    val headerChecksum: Int,
    val sourceAddress: ByteArray,
    val destAddress: ByteArray
) {
    val headerLength: Int
        get() = ihl * 4

    val sourceAddressBytes: ByteArray
        get() = sourceAddress.copyOf()

    val destAddressBytes: ByteArray
        get() = destAddress.copyOf()

    /**
     * Serialize the header back to bytes, recalculating the checksum.
     */
    fun toBytes(): ByteArray {
        val bytes = ByteArray(headerLength)

        // Byte 0: version (4 bits) + IHL (4 bits)
        bytes[0] = ((version shl 4) or ihl).toByte()

        // Byte 1: TOS
        bytes[1] = tos.toByte()

        // Bytes 2-3: Total length
        bytes[2] = (totalLength shr 8).toByte()
        bytes[3] = totalLength.toByte()

        // Bytes 4-5: Identification
        bytes[4] = (identification shr 8).toByte()
        bytes[5] = identification.toByte()

        // Bytes 6-7: Flags (3 bits) + Fragment offset (13 bits)
        val flagsAndOffset = (flags shl 13) or fragmentOffset
        bytes[6] = (flagsAndOffset shr 8).toByte()
        bytes[7] = flagsAndOffset.toByte()

        // Byte 8: TTL
        bytes[8] = ttl.toByte()

        // Byte 9: Protocol
        bytes[9] = protocol.toByte()

        // Bytes 10-11: Header checksum (set to 0 for calculation)
        bytes[10] = 0
        bytes[11] = 0

        // Bytes 12-15: Source address
        System.arraycopy(sourceAddress, 0, bytes, 12, 4)

        // Bytes 16-19: Destination address
        System.arraycopy(destAddress, 0, bytes, 16, 4)

        // Calculate and set checksum
        val checksum = Checksum.ipChecksum(bytes, 0, headerLength)
        bytes[10] = (checksum shr 8).toByte()
        bytes[11] = checksum.toByte()

        return bytes
    }

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is IPv4Header) return false
        return version == other.version &&
            ihl == other.ihl &&
            tos == other.tos &&
            totalLength == other.totalLength &&
            identification == other.identification &&
            flags == other.flags &&
            fragmentOffset == other.fragmentOffset &&
            ttl == other.ttl &&
            protocol == other.protocol &&
            headerChecksum == other.headerChecksum &&
            sourceAddress.contentEquals(other.sourceAddress) &&
            destAddress.contentEquals(other.destAddress)
    }

    override fun hashCode(): Int {
        var result = version
        result = 31 * result + ihl
        result = 31 * result + tos
        result = 31 * result + totalLength
        result = 31 * result + identification
        result = 31 * result + flags
        result = 31 * result + fragmentOffset
        result = 31 * result + ttl
        result = 31 * result + protocol
        result = 31 * result + headerChecksum
        result = 31 * result + sourceAddress.contentHashCode()
        result = 31 * result + destAddress.contentHashCode()
        return result
    }

    companion object {
        const val PROTOCOL_TCP = 6
        const val PROTOCOL_UDP = 17

        /**
         * Parse an IPv4 header from raw bytes.
         *
         * @param buffer the byte array containing the IP packet
         * @param offset the starting position within the buffer
         * @return a parsed IPv4Header
         * @throws IllegalArgumentException if the buffer is too short or the version is not 4
         */
        fun parse(buffer: ByteArray, offset: Int = 0): IPv4Header {
            require(buffer.size - offset >= 20) { "Buffer too short for IPv4 header: need 20 bytes, have ${buffer.size - offset}" }

            val versionIhl = buffer[offset].toInt() and 0xFF
            val version = versionIhl ushr 4
            val ihl = versionIhl and 0x0F

            require(version == 4) { "Not an IPv4 packet: version=$version" }
            require(ihl >= 5) { "Invalid IHL: $ihl (minimum is 5)" }
            require(buffer.size - offset >= ihl * 4) { "Buffer too short for IPv4 header with IHL=$ihl" }

            val tos = buffer[offset + 1].toInt() and 0xFF

            val totalLength = ((buffer[offset + 2].toInt() and 0xFF) shl 8) or
                (buffer[offset + 3].toInt() and 0xFF)

            val identification = ((buffer[offset + 4].toInt() and 0xFF) shl 8) or
                (buffer[offset + 5].toInt() and 0xFF)

            val flagsAndOffset = ((buffer[offset + 6].toInt() and 0xFF) shl 8) or
                (buffer[offset + 7].toInt() and 0xFF)
            val flags = flagsAndOffset ushr 13
            val fragmentOffset = flagsAndOffset and 0x1FFF

            val ttl = buffer[offset + 8].toInt() and 0xFF
            val protocol = buffer[offset + 9].toInt() and 0xFF

            val headerChecksum = ((buffer[offset + 10].toInt() and 0xFF) shl 8) or
                (buffer[offset + 11].toInt() and 0xFF)

            val sourceAddress = ByteArray(4)
            System.arraycopy(buffer, offset + 12, sourceAddress, 0, 4)

            val destAddress = ByteArray(4)
            System.arraycopy(buffer, offset + 16, destAddress, 0, 4)

            return IPv4Header(
                version = version,
                ihl = ihl,
                tos = tos,
                totalLength = totalLength,
                identification = identification,
                flags = flags,
                fragmentOffset = fragmentOffset,
                ttl = ttl,
                protocol = protocol,
                headerChecksum = headerChecksum,
                sourceAddress = sourceAddress,
                destAddress = destAddress
            )
        }
    }
}
