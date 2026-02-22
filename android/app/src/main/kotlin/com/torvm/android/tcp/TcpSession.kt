package com.torvm.android.tcp

import com.torvm.android.packet.IPv4Header
import com.torvm.android.packet.PacketBuilder
import com.torvm.android.packet.TcpHeader
import com.torvm.android.socks.Socks5Client
import com.torvm.android.tunnel.TunWriter
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import java.io.InputStream
import java.io.OutputStream
import java.net.Socket
import kotlin.random.Random

/**
 * Unique key identifying a TCP connection (4-tuple).
 *
 * Because [ByteArray] does not implement structural equality by default,
 * [equals] and [hashCode] are overridden to compare addresses by content.
 */
data class SessionKey(
    val srcAddr: ByteArray,
    val srcPort: Int,
    val dstAddr: ByteArray,
    val dstPort: Int
) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is SessionKey) return false
        return srcPort == other.srcPort &&
            dstPort == other.dstPort &&
            srcAddr.contentEquals(other.srcAddr) &&
            dstAddr.contentEquals(other.dstAddr)
    }

    override fun hashCode(): Int {
        var result = srcAddr.contentHashCode()
        result = 31 * result + srcPort
        result = 31 * result + dstAddr.contentHashCode()
        result = 31 * result + dstPort
        return result
    }
}

/**
 * Manages the state of a single proxied TCP connection.
 *
 * For each connection from an Android app the session:
 * 1. Accepts the SYN and replies with SYN+ACK.
 * 2. Connects to the real destination via SOCKS5 (through Tor).
 * 3. Relays data bidirectionally until the connection closes.
 *
 * All response packets are written back to the TUN device through [tunWriter].
 */
class TcpSession(
    val key: SessionKey,
    private val socksHost: String,
    private val socksPort: Int,
    private val tunWriter: TunWriter,
    private val protector: ((Socket) -> Boolean)?
) {
    var state: TcpState = TcpState.LISTEN
        private set

    private var clientIsn: Long = 0
    private var ourIsn: Long = Random.nextLong(0, 0xFFFFFFFFL)
    private var clientSeq: Long = 0   // next expected sequence number from the client
    private var ourSeq: Long = ourIsn + 1  // next sequence number we will send

    private var upstreamSocket: Socket? = null
    private var upstreamInput: InputStream? = null
    private var upstreamOutput: OutputStream? = null
    private var upstreamJob: Job? = null

    @Volatile
    private var upstreamConnected: Boolean = false

    var lastActivity: Long = System.currentTimeMillis()
        private set

    val isClosed: Boolean
        get() = state == TcpState.CLOSED

    companion object {
        /** Maximum segment size for data sent back to the client. */
        const val MSS = 1400

        /** Advertised receive window. */
        const val WINDOW_SIZE = 65535

        private const val TAG = "TcpSession"
    }

    // -----------------------------------------------------------------
    // Packet dispatch
    // -----------------------------------------------------------------

    /**
     * Process an incoming TCP packet for this session.
     *
     * Dispatches to the appropriate handler based on current [state] and the
     * TCP flags present in [tcpHeader].
     */
    suspend fun handlePacket(
        ipHeader: IPv4Header,
        tcpHeader: TcpHeader,
        payload: ByteArray
    ) {
        lastActivity = System.currentTimeMillis()

        when (state) {
            TcpState.LISTEN -> {
                if (tcpHeader.isSyn && !tcpHeader.isAck) {
                    handleSyn(tcpHeader)
                }
            }

            TcpState.SYN_RECEIVED -> {
                if (tcpHeader.isAck && !tcpHeader.isSyn) {
                    // Handshake ACK from client
                    if (upstreamConnected) {
                        state = TcpState.ESTABLISHED
                    }
                    // If upstream is not connected yet the session stays in
                    // SYN_RECEIVED; once connectUpstream() finishes it will
                    // set upstreamConnected = true and the next ACK (or data
                    // packet) will transition the state.
                } else if (tcpHeader.isRst) {
                    close()
                }
            }

            TcpState.ESTABLISHED -> {
                when {
                    tcpHeader.isRst -> close()
                    tcpHeader.isFin -> handleFin(tcpHeader)
                    payload.isNotEmpty() -> handleEstablishedData(tcpHeader, payload)
                    // Pure ACK with no data -- nothing to do
                }
            }

            TcpState.CLOSE_WAIT -> {
                // Waiting for our side to finish; ignore further data
                if (tcpHeader.isRst) close()
            }

            TcpState.LAST_ACK -> {
                if (tcpHeader.isAck) {
                    state = TcpState.CLOSED
                } else if (tcpHeader.isRst) {
                    close()
                }
            }

            TcpState.FIN_WAIT_1 -> {
                if (tcpHeader.isFin && tcpHeader.isAck) {
                    // Simultaneous close
                    clientSeq = (tcpHeader.sequenceNumber + 1) and 0xFFFFFFFFL
                    sendTcpToTun(TcpHeader.FLAG_ACK)
                    state = TcpState.TIME_WAIT
                } else if (tcpHeader.isAck) {
                    state = TcpState.FIN_WAIT_2
                } else if (tcpHeader.isFin) {
                    clientSeq = (tcpHeader.sequenceNumber + 1) and 0xFFFFFFFFL
                    sendTcpToTun(TcpHeader.FLAG_ACK)
                    state = TcpState.TIME_WAIT
                }
            }

            TcpState.FIN_WAIT_2 -> {
                if (tcpHeader.isFin) {
                    clientSeq = (tcpHeader.sequenceNumber + 1) and 0xFFFFFFFFL
                    sendTcpToTun(TcpHeader.FLAG_ACK)
                    state = TcpState.TIME_WAIT
                }
            }

            TcpState.TIME_WAIT, TcpState.CLOSED -> {
                // Ignore; session will be reaped by cleanup
            }
        }
    }

    // -----------------------------------------------------------------
    // SYN handling -- new connection setup
    // -----------------------------------------------------------------

    private suspend fun handleSyn(tcpHeader: TcpHeader) {
        clientIsn = tcpHeader.sequenceNumber
        clientSeq = (clientIsn + 1) and 0xFFFFFFFFL

        // Send SYN+ACK
        sendTcpToTun(
            flags = TcpHeader.FLAG_SYN or TcpHeader.FLAG_ACK,
            seqOverride = ourIsn
        )

        state = TcpState.SYN_RECEIVED

        // Begin upstream SOCKS5 connection asynchronously
        connectUpstream()
    }

    // -----------------------------------------------------------------
    // Data relay (client -> upstream)
    // -----------------------------------------------------------------

    private suspend fun handleEstablishedData(
        tcpHeader: TcpHeader,
        payload: ByteArray
    ) {
        // If we were still in SYN_RECEIVED but upstream finished connecting,
        // promote to ESTABLISHED now.
        if (state == TcpState.SYN_RECEIVED && upstreamConnected) {
            state = TcpState.ESTABLISHED
        }

        val output = upstreamOutput ?: return

        try {
            output.write(payload)
            output.flush()
        } catch (e: Exception) {
            close()
            return
        }

        clientSeq = (clientSeq + payload.size) and 0xFFFFFFFFL
        sendTcpToTun(TcpHeader.FLAG_ACK)
    }

    // -----------------------------------------------------------------
    // FIN handling -- connection teardown
    // -----------------------------------------------------------------

    private suspend fun handleFin(tcpHeader: TcpHeader) {
        // ACK the client's FIN
        clientSeq = (tcpHeader.sequenceNumber + 1) and 0xFFFFFFFFL
        sendTcpToTun(TcpHeader.FLAG_ACK)
        state = TcpState.CLOSE_WAIT

        // Close upstream write direction and send our own FIN
        try {
            upstreamSocket?.shutdownOutput()
        } catch (_: Exception) { }

        sendTcpToTun(TcpHeader.FLAG_FIN or TcpHeader.FLAG_ACK)
        ourSeq = (ourSeq + 1) and 0xFFFFFFFFL
        state = TcpState.LAST_ACK
    }

    // -----------------------------------------------------------------
    // Packet injection into TUN
    // -----------------------------------------------------------------

    /**
     * Build a TCP packet heading back to the client and write it to the TUN
     * device.
     *
     * Source and destination are swapped relative to the original client
     * packet so that the packet travels FROM the remote server TO the app.
     */
    private suspend fun sendTcpToTun(
        flags: Int,
        payload: ByteArray = byteArrayOf(),
        seqOverride: Long? = null
    ) {
        val packet = PacketBuilder.buildTcpPacket(
            srcAddr = key.dstAddr,
            dstAddr = key.srcAddr,
            srcPort = key.dstPort,
            dstPort = key.srcPort,
            seqNum = seqOverride ?: ourSeq,
            ackNum = clientSeq,
            flags = flags,
            window = WINDOW_SIZE,
            payload = payload
        )
        tunWriter.write(packet)
    }

    // -----------------------------------------------------------------
    // Upstream SOCKS5 connection
    // -----------------------------------------------------------------

    private suspend fun connectUpstream() {
        try {
            val client = Socks5Client(socksHost, socksPort)
            client.protector = protector
            val socket = client.connect(key.dstAddr, key.dstPort)

            upstreamSocket = socket
            upstreamInput = socket.getInputStream()
            upstreamOutput = socket.getOutputStream()
            upstreamConnected = true

            // If the three-way handshake ACK already arrived while we were
            // connecting, promote to ESTABLISHED now.
            if (state == TcpState.SYN_RECEIVED) {
                state = TcpState.ESTABLISHED
            }

            startUpstreamReader(CoroutineScope(Dispatchers.IO))
        } catch (e: Exception) {
            // SOCKS5 connect failed -- send RST to client
            sendTcpToTun(TcpHeader.FLAG_RST or TcpHeader.FLAG_ACK)
            state = TcpState.CLOSED
        }
    }

    // -----------------------------------------------------------------
    // Upstream reader coroutine (upstream -> client)
    // -----------------------------------------------------------------

    private fun startUpstreamReader(scope: CoroutineScope) {
        upstreamJob = scope.launch {
            val buffer = ByteArray(MSS)
            try {
                val input = upstreamInput ?: return@launch
                while (true) {
                    val bytesRead = input.read(buffer)
                    if (bytesRead == -1) break

                    val data = buffer.copyOf(bytesRead)
                    sendTcpToTun(
                        flags = TcpHeader.FLAG_PSH or TcpHeader.FLAG_ACK,
                        payload = data
                    )
                    ourSeq = (ourSeq + bytesRead) and 0xFFFFFFFFL
                    lastActivity = System.currentTimeMillis()
                }
            } catch (_: Exception) {
                // Socket closed or read error
            }

            // Upstream EOF -- send FIN to client
            if (state == TcpState.ESTABLISHED) {
                sendTcpToTun(TcpHeader.FLAG_FIN or TcpHeader.FLAG_ACK)
                ourSeq = (ourSeq + 1) and 0xFFFFFFFFL
                state = TcpState.FIN_WAIT_1
            }
        }
    }

    // -----------------------------------------------------------------
    // Cleanup
    // -----------------------------------------------------------------

    /**
     * Forcefully close the session, releasing all resources.
     */
    fun close() {
        upstreamJob?.cancel()
        try {
            upstreamSocket?.close()
        } catch (_: Exception) { }
        upstreamSocket = null
        upstreamInput = null
        upstreamOutput = null
        state = TcpState.CLOSED
    }
}
