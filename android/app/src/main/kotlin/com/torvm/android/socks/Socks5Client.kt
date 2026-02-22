package com.torvm.android.socks

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.InputStream
import java.io.OutputStream
import java.net.InetSocketAddress
import java.net.Socket

/**
 * SOCKS5 CONNECT client per RFC 1928.
 *
 * Establishes a connection through a SOCKS5 proxy server (e.g., Tor's SOCKS port)
 * to a specified destination. Supports IPv4 address connections and hostname
 * connections.
 *
 * The [protector] callback allows Android's VpnService.protect() to be injected
 * so that the proxy connection itself bypasses the VPN tunnel.
 *
 * @param proxyHost the SOCKS5 proxy hostname or IP address
 * @param proxyPort the SOCKS5 proxy port
 */
class Socks5Client(
    private val proxyHost: String,
    private val proxyPort: Int
) {
    /**
     * Optional socket protector callback. When set, this is called with the raw
     * socket before connecting to the proxy. This is used to inject
     * VpnService.protect() so the proxy connection bypasses the VPN tunnel.
     */
    var protector: ((Socket) -> Boolean)? = null

    private companion object {
        const val SOCKS_VERSION: Byte = 0x05
        const val AUTH_NONE: Byte = 0x00
        const val CMD_CONNECT: Byte = 0x01
        const val RESERVED: Byte = 0x00
        const val ADDR_TYPE_IPV4: Byte = 0x01
        const val ADDR_TYPE_DOMAIN: Byte = 0x03

        const val CONNECT_TIMEOUT_MS = 10_000
    }

    /**
     * Connect to a destination specified by hostname through the SOCKS5 proxy.
     *
     * @param destHost the destination hostname
     * @param destPort the destination port
     * @return a connected [Socket] ready for data transfer; the caller owns this socket
     * @throws Socks5Exception on SOCKS5 protocol errors or connection failures
     */
    suspend fun connect(destHost: String, destPort: Int): Socket = withContext(Dispatchers.IO) {
        val socket = Socket()
        try {
            protectAndConnect(socket)

            val output = socket.getOutputStream()
            val input = socket.getInputStream()

            performGreeting(output, input)
            sendDomainConnectRequest(output, destHost, destPort)
            readConnectReply(input)

            socket
        } catch (e: Socks5Exception) {
            socket.closeSilently()
            throw e
        } catch (e: Exception) {
            socket.closeSilently()
            throw Socks5Exception("SOCKS5 connection failed: ${e.message}", cause = e)
        }
    }

    /**
     * Connect to a destination specified by IPv4 address through the SOCKS5 proxy.
     *
     * @param destAddr the destination IPv4 address (4 bytes)
     * @param destPort the destination port
     * @return a connected [Socket] ready for data transfer; the caller owns this socket
     * @throws Socks5Exception on SOCKS5 protocol errors or connection failures
     */
    suspend fun connect(destAddr: ByteArray, destPort: Int): Socket = withContext(Dispatchers.IO) {
        require(destAddr.size == 4) { "destAddr must be 4 bytes for IPv4" }

        val socket = Socket()
        try {
            protectAndConnect(socket)

            val output = socket.getOutputStream()
            val input = socket.getInputStream()

            performGreeting(output, input)
            sendIpv4ConnectRequest(output, destAddr, destPort)
            readConnectReply(input)

            socket
        } catch (e: Socks5Exception) {
            socket.closeSilently()
            throw e
        } catch (e: Exception) {
            socket.closeSilently()
            throw Socks5Exception("SOCKS5 connection failed: ${e.message}", cause = e)
        }
    }

    /**
     * Protect the socket (if a protector is set) and connect to the SOCKS5 proxy.
     */
    private fun protectAndConnect(socket: Socket) {
        val prot = protector
        if (prot != null) {
            if (!prot(socket)) {
                throw Socks5Exception("Socket protection failed")
            }
        }
        socket.connect(InetSocketAddress(proxyHost, proxyPort), CONNECT_TIMEOUT_MS)
    }

    /**
     * Perform the SOCKS5 greeting/authentication negotiation.
     *
     * Sends: [0x05, 0x01, 0x00] (version 5, 1 auth method, no-auth)
     * Expects: [0x05, 0x00] (version 5, no-auth selected)
     */
    private fun performGreeting(output: OutputStream, input: InputStream) {
        // Send greeting: version 5, 1 method offered, no-auth
        output.write(byteArrayOf(SOCKS_VERSION, 0x01, AUTH_NONE))
        output.flush()

        // Read 2-byte response
        val response = readExact(input, 2)

        if (response[0] != SOCKS_VERSION) {
            throw Socks5Exception(
                "Unexpected SOCKS version in greeting response: ${response[0].toInt() and 0xFF}"
            )
        }
        if (response[1] != AUTH_NONE) {
            throw Socks5Exception(
                "SOCKS5 server did not accept no-auth method: ${response[1].toInt() and 0xFF}"
            )
        }
    }

    /**
     * Send a SOCKS5 CONNECT request for an IPv4 address.
     *
     * Format: [0x05, 0x01, 0x00, 0x01, <4-byte addr>, <2-byte port BE>]
     */
    private fun sendIpv4ConnectRequest(
        output: OutputStream,
        destAddr: ByteArray,
        destPort: Int
    ) {
        val request = ByteArray(10)
        request[0] = SOCKS_VERSION
        request[1] = CMD_CONNECT
        request[2] = RESERVED
        request[3] = ADDR_TYPE_IPV4
        System.arraycopy(destAddr, 0, request, 4, 4)
        request[8] = (destPort shr 8).toByte()
        request[9] = destPort.toByte()

        output.write(request)
        output.flush()
    }

    /**
     * Send a SOCKS5 CONNECT request for a domain name.
     *
     * Format: [0x05, 0x01, 0x00, 0x03, <len>, <domain bytes>, <2-byte port BE>]
     */
    private fun sendDomainConnectRequest(
        output: OutputStream,
        destHost: String,
        destPort: Int
    ) {
        val hostBytes = destHost.toByteArray(Charsets.US_ASCII)
        require(hostBytes.size <= 255) { "Domain name too long: ${hostBytes.size} bytes (max 255)" }

        val request = ByteArray(4 + 1 + hostBytes.size + 2)
        request[0] = SOCKS_VERSION
        request[1] = CMD_CONNECT
        request[2] = RESERVED
        request[3] = ADDR_TYPE_DOMAIN
        request[4] = hostBytes.size.toByte()
        System.arraycopy(hostBytes, 0, request, 5, hostBytes.size)
        request[5 + hostBytes.size] = (destPort shr 8).toByte()
        request[6 + hostBytes.size] = destPort.toByte()

        output.write(request)
        output.flush()
    }

    /**
     * Read and validate the SOCKS5 CONNECT reply.
     *
     * The minimum reply for IPv4 is 10 bytes:
     * [version, reply, reserved, addr_type, 4-byte addr, 2-byte port]
     *
     * For domain replies the length varies. We handle IPv4 (0x01), domain (0x03),
     * and IPv6 (0x04) address types to consume the full reply from the stream.
     */
    private fun readConnectReply(input: InputStream) {
        // Read the first 4 bytes: version, reply, reserved, address type
        val header = readExact(input, 4)

        if (header[0] != SOCKS_VERSION) {
            throw Socks5Exception(
                "Unexpected SOCKS version in connect reply: ${header[0].toInt() and 0xFF}"
            )
        }

        val replyCode = header[1].toInt() and 0xFF
        if (replyCode != 0x00) {
            throw Socks5Exception(
                "SOCKS5 connect failed: reply code 0x${replyCode.toString(16).padStart(2, '0')}",
                replyCode = replyCode
            )
        }

        // Consume the bound address and port based on address type
        val addrType = header[3].toInt() and 0xFF
        when (addrType) {
            0x01 -> {
                // IPv4: 4 bytes address + 2 bytes port
                readExact(input, 6)
            }
            0x03 -> {
                // Domain: 1 byte length + N bytes domain + 2 bytes port
                val lenBuf = readExact(input, 1)
                val domainLen = lenBuf[0].toInt() and 0xFF
                readExact(input, domainLen + 2)
            }
            0x04 -> {
                // IPv6: 16 bytes address + 2 bytes port
                readExact(input, 18)
            }
            else -> {
                throw Socks5Exception("Unknown SOCKS5 address type in reply: 0x${addrType.toString(16)}")
            }
        }
    }

    /**
     * Read exactly [count] bytes from the input stream.
     *
     * @throws Socks5Exception if the stream ends before all bytes are read
     */
    private fun readExact(input: InputStream, count: Int): ByteArray {
        val buffer = ByteArray(count)
        var totalRead = 0
        while (totalRead < count) {
            val bytesRead = input.read(buffer, totalRead, count - totalRead)
            if (bytesRead == -1) {
                throw Socks5Exception(
                    "SOCKS5 connection closed unexpectedly: read $totalRead of $count bytes"
                )
            }
            totalRead += bytesRead
        }
        return buffer
    }

    /**
     * Close a socket without throwing exceptions.
     */
    private fun Socket.closeSilently() {
        try {
            close()
        } catch (_: Exception) {
            // Intentionally ignored
        }
    }
}
