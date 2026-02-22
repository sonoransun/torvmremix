package com.torvm.android.packet

/**
 * Utility object for computing IP and TCP/UDP checksums per RFC 791.
 *
 * All checksums use the standard one's complement sum algorithm:
 * sum all 16-bit words, fold any carries back in, then take the one's complement.
 */
object Checksum {

    /**
     * Compute the standard RFC 791 IP checksum over the given byte range.
     *
     * The algorithm:
     * 1. Sum all 16-bit words in the data.
     * 2. If there is an odd byte at the end, pad it with a zero byte and add.
     * 3. Fold any carry bits (upper 16 bits) back into the lower 16 bits.
     * 4. Take the one's complement of the result.
     *
     * @param data the byte array to checksum
     * @param offset starting position within the array
     * @param length number of bytes to include
     * @return the 16-bit checksum value
     */
    fun ipChecksum(data: ByteArray, offset: Int = 0, length: Int = data.size): Int {
        var sum = 0L
        var i = 0

        // Sum 16-bit words
        while (i + 1 < length) {
            val word = ((data[offset + i].toInt() and 0xFF) shl 8) or
                (data[offset + i + 1].toInt() and 0xFF)
            sum += word
            i += 2
        }

        // If odd number of bytes, pad the last byte with zero
        if (i < length) {
            sum += (data[offset + i].toInt() and 0xFF) shl 8
        }

        // Fold 32-bit sum into 16 bits
        while (sum shr 16 != 0L) {
            sum = (sum and 0xFFFF) + (sum shr 16)
        }

        // One's complement
        return (sum.toInt().inv()) and 0xFFFF
    }

    /**
     * Compute the TCP or UDP checksum using the IPv4 pseudo-header.
     *
     * The pseudo-header consists of:
     * - Source address (4 bytes)
     * - Destination address (4 bytes)
     * - Zero byte (1 byte)
     * - Protocol number (1 byte)
     * - TCP/UDP segment length (2 bytes, big-endian)
     *
     * This is concatenated with the transport header and payload, then the
     * standard one's complement checksum is computed over the whole thing.
     *
     * @param ipHeader the IPv4 header (provides source/dest addresses and protocol)
     * @param transportHeader the TCP or UDP header bytes (with checksum field zeroed)
     * @param payload the data payload following the transport header
     * @return the 16-bit checksum value
     */
    fun tcpUdpChecksum(
        ipHeader: IPv4Header,
        transportHeader: ByteArray,
        payload: ByteArray = byteArrayOf()
    ): Int {
        val segmentLength = transportHeader.size + payload.size

        // Build pseudo-header (12 bytes) + transport header + payload
        val totalLength = 12 + segmentLength
        val buffer = ByteArray(totalLength)

        // Pseudo-header: source address (4 bytes)
        System.arraycopy(ipHeader.sourceAddress, 0, buffer, 0, 4)

        // Pseudo-header: destination address (4 bytes)
        System.arraycopy(ipHeader.destAddress, 0, buffer, 4, 4)

        // Pseudo-header: zero + protocol + segment length
        buffer[8] = 0
        buffer[9] = ipHeader.protocol.toByte()
        buffer[10] = (segmentLength shr 8).toByte()
        buffer[11] = segmentLength.toByte()

        // Transport header
        System.arraycopy(transportHeader, 0, buffer, 12, transportHeader.size)

        // Payload
        if (payload.isNotEmpty()) {
            System.arraycopy(payload, 0, buffer, 12 + transportHeader.size, payload.size)
        }

        return ipChecksum(buffer)
    }
}
