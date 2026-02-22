package com.torvm.android.packet

/**
 * Parses and builds TCP headers.
 *
 * Sequence and acknowledgment numbers are stored as Long to properly handle
 * unsigned 32-bit values without sign issues.
 *
 * All multi-byte fields use network byte order (big-endian).
 */
data class TcpHeader(
    val sourcePort: Int,
    val destPort: Int,
    val sequenceNumber: Long,
    val ackNumber: Long,
    val dataOffset: Int,
    val reserved: Int,
    val flags: Int,
    val window: Int,
    val checksum: Int,
    val urgentPointer: Int
) {
    val headerLength: Int
        get() = dataOffset * 4

    val isSyn: Boolean get() = (flags and FLAG_SYN) != 0
    val isAck: Boolean get() = (flags and FLAG_ACK) != 0
    val isFin: Boolean get() = (flags and FLAG_FIN) != 0
    val isRst: Boolean get() = (flags and FLAG_RST) != 0
    val isPsh: Boolean get() = (flags and FLAG_PSH) != 0

    /**
     * Serialize the TCP header to bytes.
     *
     * Note: The checksum field is written as-is. Callers should compute the
     * correct checksum using [Checksum.tcpUdpChecksum] after building the header.
     */
    fun toBytes(): ByteArray {
        val bytes = ByteArray(headerLength)

        // Bytes 0-1: Source port
        bytes[0] = (sourcePort shr 8).toByte()
        bytes[1] = sourcePort.toByte()

        // Bytes 2-3: Destination port
        bytes[2] = (destPort shr 8).toByte()
        bytes[3] = destPort.toByte()

        // Bytes 4-7: Sequence number (unsigned 32-bit as Long)
        val seq = sequenceNumber and 0xFFFFFFFFL
        bytes[4] = (seq shr 24).toByte()
        bytes[5] = (seq shr 16).toByte()
        bytes[6] = (seq shr 8).toByte()
        bytes[7] = seq.toByte()

        // Bytes 8-11: Acknowledgment number (unsigned 32-bit as Long)
        val ack = ackNumber and 0xFFFFFFFFL
        bytes[8] = (ack shr 24).toByte()
        bytes[9] = (ack shr 16).toByte()
        bytes[10] = (ack shr 8).toByte()
        bytes[11] = ack.toByte()

        // Byte 12: Data offset (4 bits) + reserved (3 bits) + NS flag (1 bit)
        // Byte 13: Remaining 8 flag bits (CWR, ECE, URG, ACK, PSH, RST, SYN, FIN)
        val ns = (flags shr 8) and 0x01
        bytes[12] = ((dataOffset shl 4) or (reserved shl 1) or ns).toByte()
        bytes[13] = (flags and 0xFF).toByte()

        // Bytes 14-15: Window size
        bytes[14] = (window shr 8).toByte()
        bytes[15] = window.toByte()

        // Bytes 16-17: Checksum
        bytes[16] = (checksum shr 8).toByte()
        bytes[17] = checksum.toByte()

        // Bytes 18-19: Urgent pointer
        bytes[18] = (urgentPointer shr 8).toByte()
        bytes[19] = urgentPointer.toByte()

        return bytes
    }

    companion object {
        const val FLAG_FIN = 0x001
        const val FLAG_SYN = 0x002
        const val FLAG_RST = 0x004
        const val FLAG_PSH = 0x008
        const val FLAG_ACK = 0x010
        const val FLAG_URG = 0x020
        const val FLAG_ECE = 0x040
        const val FLAG_CWR = 0x080
        const val FLAG_NS  = 0x100

        /**
         * Parse a TCP header from raw bytes.
         *
         * @param buffer the byte array containing the TCP segment
         * @param offset the starting position of the TCP header within the buffer
         * @return a parsed TcpHeader
         * @throws IllegalArgumentException if the buffer is too short
         */
        fun parse(buffer: ByteArray, offset: Int): TcpHeader {
            require(buffer.size - offset >= 20) { "Buffer too short for TCP header: need 20 bytes, have ${buffer.size - offset}" }

            val sourcePort = ((buffer[offset].toInt() and 0xFF) shl 8) or
                (buffer[offset + 1].toInt() and 0xFF)

            val destPort = ((buffer[offset + 2].toInt() and 0xFF) shl 8) or
                (buffer[offset + 3].toInt() and 0xFF)

            val sequenceNumber = ((buffer[offset + 4].toLong() and 0xFF) shl 24) or
                ((buffer[offset + 5].toLong() and 0xFF) shl 16) or
                ((buffer[offset + 6].toLong() and 0xFF) shl 8) or
                (buffer[offset + 7].toLong() and 0xFF)

            val ackNumber = ((buffer[offset + 8].toLong() and 0xFF) shl 24) or
                ((buffer[offset + 9].toLong() and 0xFF) shl 16) or
                ((buffer[offset + 10].toLong() and 0xFF) shl 8) or
                (buffer[offset + 11].toLong() and 0xFF)

            val byte12 = buffer[offset + 12].toInt() and 0xFF
            val dataOffset = byte12 ushr 4
            val reserved = (byte12 shr 1) and 0x07
            val ns = byte12 and 0x01

            val byte13 = buffer[offset + 13].toInt() and 0xFF
            val flags = (ns shl 8) or byte13

            val window = ((buffer[offset + 14].toInt() and 0xFF) shl 8) or
                (buffer[offset + 15].toInt() and 0xFF)

            val checksum = ((buffer[offset + 16].toInt() and 0xFF) shl 8) or
                (buffer[offset + 17].toInt() and 0xFF)

            val urgentPointer = ((buffer[offset + 18].toInt() and 0xFF) shl 8) or
                (buffer[offset + 19].toInt() and 0xFF)

            require(dataOffset >= 5) { "Invalid TCP data offset: $dataOffset (minimum is 5)" }

            return TcpHeader(
                sourcePort = sourcePort,
                destPort = destPort,
                sequenceNumber = sequenceNumber,
                ackNumber = ackNumber,
                dataOffset = dataOffset,
                reserved = reserved,
                flags = flags,
                window = window,
                checksum = checksum,
                urgentPointer = urgentPointer
            )
        }
    }
}
