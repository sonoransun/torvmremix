import Foundation

/// Unique key identifying a TCP connection (4-tuple).
///
/// Address bytes are stored as `Data` (typically 4 bytes for IPv4).
/// Conforms to `Hashable` so it can be used as a dictionary key.
struct SessionKey: Hashable {
    let srcAddr: Data
    let srcPort: UInt16
    let dstAddr: Data
    let dstPort: UInt16

    func hash(into hasher: inout Hasher) {
        hasher.combine(srcAddr)
        hasher.combine(srcPort)
        hasher.combine(dstAddr)
        hasher.combine(dstPort)
    }

    static func == (lhs: SessionKey, rhs: SessionKey) -> Bool {
        return lhs.srcPort == rhs.srcPort &&
            lhs.dstPort == rhs.dstPort &&
            lhs.srcAddr == rhs.srcAddr &&
            lhs.dstAddr == rhs.dstAddr
    }
}
