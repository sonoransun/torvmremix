package com.torvm.android.dns

import com.torvm.android.packet.IPv4Header
import com.torvm.android.packet.PacketBuilder
import com.torvm.android.packet.UdpHeader
import com.torvm.android.tunnel.TunWriter
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress
import java.util.concurrent.Semaphore

/**
 * Relays DNS queries to an external DNS resolver (typically Tor's DNSPort)
 * and injects the responses back into the TUN device.
 *
 * Each query is forwarded as a plain UDP datagram. The [protector] lambda
 * must be invoked on the socket so that the VpnService marks it as
 * "protected" -- preventing the DNS traffic itself from being routed back
 * through the TUN and creating an infinite loop.
 */
class DnsRelay(
    private val dnsHost: String,
    private val dnsPort: Int,
    private val tunWriter: TunWriter,
    private val protector: ((DatagramSocket) -> Boolean)?
) {
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())
    private val concurrencyLimit = Semaphore(MAX_CONCURRENT_QUERIES)

    companion object {
        /** Maximum size of a DNS UDP response. */
        private const val DNS_BUFFER_SIZE = 4096

        /** Socket timeout for upstream DNS queries. */
        private const val DNS_TIMEOUT_MS = 5000

        /** Maximum number of concurrent DNS queries. */
        private const val MAX_CONCURRENT_QUERIES = 32

        private const val TAG = "DnsRelay"
    }

    /**
     * Start the relay.
     *
     * The relay is stateless -- this method is a no-op provided for symmetry
     * with [stop].
     */
    fun start() {
        // No-op; DNS relay handles each packet independently
    }

    /**
     * Cancel all in-flight DNS queries.
     */
    fun stop() {
        scope.cancel()
    }

    /**
     * Handle an intercepted DNS query packet.
     *
     * The query [payload] (raw DNS wire format) is forwarded to the
     * configured resolver. On success the DNS response is wrapped in a
     * UDP/IP packet with source and destination swapped and written back
     * to the TUN device.
     */
    suspend fun handlePacket(
        ipHeader: IPv4Header,
        udpHeader: UdpHeader,
        payload: ByteArray
    ) {
        scope.launch {
            if (!concurrencyLimit.tryAcquire()) return@launch
            try {
                val dnsResponse = forwardDnsQuery(payload)

                val responsePacket = PacketBuilder.buildUdpPacket(
                    srcAddr = ipHeader.destAddressBytes,
                    dstAddr = ipHeader.sourceAddressBytes,
                    srcPort = udpHeader.destPort,
                    dstPort = udpHeader.sourcePort,
                    payload = dnsResponse
                )

                tunWriter.write(responsePacket)
            } catch (_: Exception) {
                // Query failed (timeout, network error, etc.) -- drop silently.
                // The application will retry or fall back to TCP DNS.
            } finally {
                concurrencyLimit.release()
            }
        }
    }

    /**
     * Forward a raw DNS query to the upstream resolver and return the
     * response bytes.
     */
    private fun forwardDnsQuery(query: ByteArray): ByteArray {
        require(query.size >= 2) { "DNS query too short to contain transaction ID" }

        val socket = DatagramSocket()
        try {
            protector?.invoke(socket)
            socket.soTimeout = DNS_TIMEOUT_MS

            val address = InetAddress.getByName(dnsHost)
            socket.send(DatagramPacket(query, query.size, address, dnsPort))

            val buffer = ByteArray(DNS_BUFFER_SIZE)
            val response = DatagramPacket(buffer, buffer.size)
            socket.receive(response)

            // Validate that the DNS transaction ID in the response matches the query
            if (response.length < 2 ||
                buffer[0] != query[0] || buffer[1] != query[1]
            ) {
                throw IllegalStateException("DNS response transaction ID mismatch")
            }

            return buffer.copyOf(response.length)
        } finally {
            socket.close()
        }
    }
}
