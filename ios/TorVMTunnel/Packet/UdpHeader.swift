import Foundation

/// Parses and builds 8-byte UDP headers.
///
/// All multi-byte fields use network byte order (big-endian).
struct UdpHeader {
    static let headerSize = 8

    let sourcePort: UInt16
    let destPort: UInt16
    let length: UInt16
    let checksum: UInt16

    /// Serialize the UDP header to bytes.
    ///
    /// The checksum field is written as-is. Callers should compute the
    /// correct checksum using `Checksum.tcpUdpChecksum` after building the header.
    func toBytes() -> Data {
        var bytes = Data(count: 8)

        // Bytes 0-1: Source port
        bytes[0] = UInt8(sourcePort >> 8)
        bytes[1] = UInt8(sourcePort & 0xFF)

        // Bytes 2-3: Destination port
        bytes[2] = UInt8(destPort >> 8)
        bytes[3] = UInt8(destPort & 0xFF)

        // Bytes 4-5: Length
        bytes[4] = UInt8(length >> 8)
        bytes[5] = UInt8(length & 0xFF)

        // Bytes 6-7: Checksum
        bytes[6] = UInt8(checksum >> 8)
        bytes[7] = UInt8(checksum & 0xFF)

        return bytes
    }

    /// Parse a UDP header from raw bytes.
    ///
    /// - Parameters:
    ///   - data: the data containing the UDP datagram
    ///   - offset: the starting position of the UDP header within the data
    /// - Returns: a parsed UdpHeader
    /// - Throws: PacketError if the buffer is too short
    static func parse(data: Data, offset: Int) throws -> UdpHeader {
        guard data.count - offset >= 8 else {
            throw PacketError.bufferTooShort(needed: 8, have: data.count - offset)
        }

        let sourcePort = (UInt16(data[offset]) << 8) |
            UInt16(data[offset + 1])

        let destPort = (UInt16(data[offset + 2]) << 8) |
            UInt16(data[offset + 3])

        let length = (UInt16(data[offset + 4]) << 8) |
            UInt16(data[offset + 5])

        let checksum = (UInt16(data[offset + 6]) << 8) |
            UInt16(data[offset + 7])

        return UdpHeader(
            sourcePort: sourcePort,
            destPort: destPort,
            length: length,
            checksum: checksum
        )
    }
}
