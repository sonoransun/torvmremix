package com.torvm.android.tcp

import com.torvm.android.packet.IPv4Header
import com.torvm.android.packet.PacketBuilder
import com.torvm.android.packet.TcpHeader
import com.torvm.android.tunnel.TunWriter
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.net.Socket
import java.util.concurrent.ConcurrentHashMap

/**
 * Manages all active [TcpSession] instances.
 *
 * Incoming TCP packets are routed to the appropriate session based on their
 * 4-tuple [SessionKey]. New SYN packets create a fresh session; packets for
 * unknown sessions receive a RST.
 *
 * A background cleanup job periodically reaps sessions that are CLOSED,
 * stuck in TIME_WAIT beyond 60 seconds, or idle for longer than 5 minutes.
 */
class TcpSessionManager(
    private val socksHost: String,
    private val socksPort: Int,
    private val tunWriter: TunWriter,
    private val protector: ((Socket) -> Boolean)?
) {
    private val sessions = ConcurrentHashMap<SessionKey, TcpSession>()
    private val scope = CoroutineScope(
        Dispatchers.IO.limitedParallelism(64) + SupervisorJob()
    )
    private var cleanupJob: Job? = null

    companion object {
        /** Maximum number of concurrent sessions. */
        private const val MAX_SESSIONS = 1024

        /** Interval between cleanup sweeps. */
        private const val CLEANUP_INTERVAL_MS = 30_000L

        /** TIME_WAIT sessions are removed after this duration. */
        private const val TIME_WAIT_TIMEOUT_MS = 60_000L

        /** Sessions with no activity for this long are forcefully closed. */
        private const val IDLE_TIMEOUT_MS = 300_000L  // 5 minutes

        /** SYN_RECEIVED sessions are reaped after this duration. */
        private const val SYN_RECEIVED_TIMEOUT_MS = 10_000L

        private const val TAG = "TcpSessionManager"
    }

    /**
     * Start the periodic session cleanup job.
     */
    fun start() {
        cleanupJob = scope.launch {
            while (isActive) {
                delay(CLEANUP_INTERVAL_MS)
                cleanup()
            }
        }
    }

    /**
     * Stop all sessions and cancel the cleanup job.
     */
    fun stop() {
        cleanupJob?.cancel()
        cleanupJob = null
        for (session in sessions.values) {
            session.close()
        }
        sessions.clear()
        scope.cancel()
    }

    /**
     * Route a TCP packet to the correct session.
     *
     * - SYN (without ACK) creates a new session.
     * - All other packets are forwarded to the existing session.
     * - Packets for unknown sessions trigger a RST reply.
     */
    suspend fun handlePacket(
        ipHeader: IPv4Header,
        tcpHeader: TcpHeader,
        payload: ByteArray
    ) {
        val key = SessionKey(
            srcAddr = ipHeader.sourceAddressBytes,
            srcPort = tcpHeader.sourcePort,
            dstAddr = ipHeader.destAddressBytes,
            dstPort = tcpHeader.destPort
        )

        if (tcpHeader.isSyn && !tcpHeader.isAck) {
            // Enforce session limit to prevent SYN flood exhaustion
            if (sessions.size >= MAX_SESSIONS) {
                sendRst(ipHeader, tcpHeader)
                return
            }

            // New connection -- remove any stale session with the same key
            sessions.remove(key)?.close()

            val session = TcpSession(key, socksHost, socksPort, tunWriter, protector)
            sessions[key] = session
            session.handlePacket(ipHeader, tcpHeader, payload)
        } else {
            val session = sessions[key]
            if (session != null) {
                session.handlePacket(ipHeader, tcpHeader, payload)
                if (session.isClosed) {
                    sessions.remove(key)
                }
            } else {
                // No session found -- send RST so the client stops retrying
                sendRst(ipHeader, tcpHeader)
            }
        }
    }

    /**
     * Send a RST packet in response to a packet for an unknown session.
     */
    private suspend fun sendRst(ipHeader: IPv4Header, tcpHeader: TcpHeader) {
        val ackNum = if (tcpHeader.isSyn) {
            (tcpHeader.sequenceNumber + 1) and 0xFFFFFFFFL
        } else {
            tcpHeader.sequenceNumber
        }

        val packet = PacketBuilder.buildTcpPacket(
            srcAddr = ipHeader.destAddressBytes,
            dstAddr = ipHeader.sourceAddressBytes,
            srcPort = tcpHeader.destPort,
            dstPort = tcpHeader.sourcePort,
            seqNum = tcpHeader.ackNumber,
            ackNum = ackNum,
            flags = TcpHeader.FLAG_RST or TcpHeader.FLAG_ACK,
            window = 0,
            payload = byteArrayOf()
        )
        tunWriter.write(packet)
    }

    /**
     * Remove sessions that are CLOSED, timed-out in TIME_WAIT, or idle too
     * long.
     */
    private fun cleanup() {
        val now = System.currentTimeMillis()
        val iterator = sessions.entries.iterator()

        while (iterator.hasNext()) {
            val (_, session) = iterator.next()
            val elapsed = now - session.lastActivity

            val shouldRemove = when (session.state) {
                TcpState.CLOSED -> true
                TcpState.TIME_WAIT -> elapsed > TIME_WAIT_TIMEOUT_MS
                TcpState.SYN_RECEIVED -> elapsed > SYN_RECEIVED_TIMEOUT_MS
                else -> elapsed > IDLE_TIMEOUT_MS
            }

            if (shouldRemove) {
                session.close()
                iterator.remove()
            }
        }
    }
}
