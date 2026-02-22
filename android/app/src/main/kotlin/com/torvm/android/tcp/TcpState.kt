package com.torvm.android.tcp

/**
 * States for the server-side TCP state machine.
 *
 * This mirrors a subset of the RFC 793 state diagram, covering the states
 * relevant to a VPN proxy that accepts connections from local apps and
 * relays them through a SOCKS5 upstream.
 */
enum class TcpState {
    LISTEN,
    SYN_RECEIVED,
    ESTABLISHED,
    CLOSE_WAIT,
    LAST_ACK,
    FIN_WAIT_1,
    FIN_WAIT_2,
    TIME_WAIT,
    CLOSED
}
