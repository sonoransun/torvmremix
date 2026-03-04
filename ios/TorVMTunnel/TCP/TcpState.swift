import Foundation

/// States for the server-side TCP state machine.
///
/// Mirrors a subset of the RFC 793 state diagram, covering the states
/// relevant to a VPN proxy that accepts connections from local apps and
/// relays them through a SOCKS5 upstream.
enum TcpState {
    case listen
    case synReceived
    case established
    case closeWait
    case lastAck
    case finWait1
    case finWait2
    case timeWait
    case closed
}
