package com.torvm.android.tunnel

import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.io.FileOutputStream

/**
 * Thread-safe writer for the TUN device file descriptor.
 *
 * Multiple coroutines (upstream TCP readers, DNS relay) write response packets
 * concurrently. A [Mutex] serializes access so that each packet is written
 * atomically.
 */
class TunWriter(private val outputStream: FileOutputStream) {

    private val mutex = Mutex()

    /**
     * Write a complete IP packet to the TUN device.
     *
     * The call suspends until the mutex is acquired, then writes and flushes
     * the packet bytes.
     */
    suspend fun write(packet: ByteArray) {
        mutex.withLock {
            outputStream.write(packet)
            outputStream.flush()
        }
    }
}
