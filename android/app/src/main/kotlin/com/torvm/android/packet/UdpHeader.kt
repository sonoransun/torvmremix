package com.torvm.android.packet

/**
 * Parses and builds 8-byte UDP headers.
 *
 * All multi-byte fields use network byte order (big-endian).
 */
data class UdpHeader(
    val sourcePort: Int,
    val destPort: Int,
    val length: Int,
    val checksum: Int
) {
    /**
     * Serialize the UDP header to bytes.
     *
     * Note: The checksum field is written as-is. Callers should compute the
     * correct checksum using [Checksum.tcpUdpChecksum] after building the header.
     */
    fun toBytes(): ByteArray {
        val bytes = ByteArray(8)

        // Bytes 0-1: Source port
        bytes[0] = (sourcePort shr 8).toByte()
        bytes[1] = sourcePort.toByte()

        // Bytes 2-3: Destination port
        bytes[2] = (destPort shr 8).toByte()
        bytes[3] = destPort.toByte()

        // Bytes 4-5: Length
        bytes[4] = (length shr 8).toByte()
        bytes[5] = length.toByte()

        // Bytes 6-7: Checksum
        bytes[6] = (checksum shr 8).toByte()
        bytes[7] = checksum.toByte()

        return bytes
    }

    companion object {
        const val HEADER_SIZE = 8

        /**
         * Parse a UDP header from raw bytes.
         *
         * @param buffer the byte array containing the UDP datagram
         * @param offset the starting position of the UDP header within the buffer
         * @return a parsed UdpHeader
         * @throws IllegalArgumentException if the buffer is too short
         */
        fun parse(buffer: ByteArray, offset: Int): UdpHeader {
            require(buffer.size - offset >= 8) { "Buffer too short for UDP header: need 8 bytes, have ${buffer.size - offset}" }

            val sourcePort = ((buffer[offset].toInt() and 0xFF) shl 8) or
                (buffer[offset + 1].toInt() and 0xFF)

            val destPort = ((buffer[offset + 2].toInt() and 0xFF) shl 8) or
                (buffer[offset + 3].toInt() and 0xFF)

            val length = ((buffer[offset + 4].toInt() and 0xFF) shl 8) or
                (buffer[offset + 5].toInt() and 0xFF)

            val checksum = ((buffer[offset + 6].toInt() and 0xFF) shl 8) or
                (buffer[offset + 7].toInt() and 0xFF)

            return UdpHeader(
                sourcePort = sourcePort,
                destPort = destPort,
                length = length,
                checksum = checksum
            )
        }
    }
}
