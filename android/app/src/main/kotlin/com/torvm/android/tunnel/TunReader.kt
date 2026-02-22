package com.torvm.android.tunnel

import java.io.FileInputStream

/**
 * Simple wrapper around the TUN file descriptor input stream.
 *
 * Performs blocking reads from the TUN device. Each successful read returns
 * one complete IP packet.
 */
class TunReader(private val inputStream: FileInputStream) {

    /**
     * Read the next packet from the TUN device into [buffer].
     *
     * @param buffer destination buffer; must be large enough to hold an IP packet (typically 32768 bytes)
     * @return the number of bytes read, or -1 on EOF
     */
    fun read(buffer: ByteArray): Int {
        return inputStream.read(buffer)
    }
}
