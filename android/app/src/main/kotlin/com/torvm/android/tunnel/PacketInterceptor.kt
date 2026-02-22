package com.torvm.android.tunnel

import com.torvm.android.dns.DnsRelay
import com.torvm.android.packet.IPv4Header
import com.torvm.android.packet.TcpHeader
import com.torvm.android.packet.UdpHeader
import com.torvm.android.tcp.TcpSessionManager

/**
 * Classifies raw IP packets read from the TUN device and dispatches them
 * to the appropriate handler.
 *
 * - **TCP packets** are forwarded to the [TcpSessionManager] which manages
 *   per-connection SOCKS5 proxying through Tor.
 * - **UDP packets to port 53** are forwarded to the [DnsRelay] which
 *   resolves them via Tor's DNSPort.
 * - **All other UDP** is silently dropped because Tor does not support
 *   generic UDP transport.
 */
class PacketInterceptor(
    private val tcpSessionManager: TcpSessionManager,
    private val dnsRelay: DnsRelay
) {
    companion object {
        /** Minimum size of an IPv4 header (no options). */
        private const val MIN_IP_HEADER_SIZE = 20

        /** Minimum size of a TCP header (no options). */
        private const val MIN_TCP_HEADER_SIZE = 20

        /** Size of a UDP header. */
        private const val UDP_HEADER_SIZE = 8

        /** Well-known DNS port. */
        private const val DNS_PORT = 53
    }

    /**
     * Parse and dispatch a single IP packet.
     *
     * @param packet raw bytes read from the TUN device
     * @param length number of valid bytes in [packet]
     */
    suspend fun intercept(packet: ByteArray, length: Int) {
        if (length < MIN_IP_HEADER_SIZE) return

        // Check IP version nibble before attempting to parse
        val version = (packet[0].toInt() shr 4) and 0x0F
        if (version != 4) return

        val ipHeader: IPv4Header
        try {
            ipHeader = IPv4Header.parse(packet)
        } catch (_: IllegalArgumentException) {
            return  // Malformed -- drop
        }

        val transportOffset = ipHeader.headerLength

        when (ipHeader.protocol) {
            IPv4Header.PROTOCOL_TCP -> {
                if (length < transportOffset + MIN_TCP_HEADER_SIZE) return

                val tcpHeader: TcpHeader
                try {
                    tcpHeader = TcpHeader.parse(packet, transportOffset)
                } catch (_: IllegalArgumentException) {
                    return  // Malformed TCP -- drop
                }

                val payloadOffset = transportOffset + tcpHeader.headerLength
                val payload = if (payloadOffset < length) {
                    packet.copyOfRange(payloadOffset, length)
                } else {
                    byteArrayOf()
                }

                tcpSessionManager.handlePacket(ipHeader, tcpHeader, payload)
            }

            IPv4Header.PROTOCOL_UDP -> {
                if (length < transportOffset + UDP_HEADER_SIZE) return

                val udpHeader: UdpHeader
                try {
                    udpHeader = UdpHeader.parse(packet, transportOffset)
                } catch (_: IllegalArgumentException) {
                    return  // Malformed UDP -- drop
                }

                if (udpHeader.destPort == DNS_PORT) {
                    val payloadOffset = transportOffset + UDP_HEADER_SIZE
                    val payload = if (payloadOffset < length) {
                        packet.copyOfRange(payloadOffset, length)
                    } else {
                        return  // Empty DNS query -- drop
                    }

                    dnsRelay.handlePacket(ipHeader, udpHeader, payload)
                }
                // Non-DNS UDP: dropped silently (Tor does not support UDP)
            }

            // Other protocols (ICMP, etc.): dropped silently
        }
    }
}
