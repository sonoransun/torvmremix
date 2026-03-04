import Foundation

/// Shared error type for packet parsing failures.
enum PacketError: Error, CustomStringConvertible {
    case bufferTooShort(needed: Int, have: Int)
    case invalidVersion(UInt8)
    case invalidIHL(UInt8)
    case invalidDataOffset(UInt8)

    var description: String {
        switch self {
        case .bufferTooShort(let needed, let have):
            return "Buffer too short: need \(needed) bytes, have \(have)"
        case .invalidVersion(let v):
            return "Not an IPv4 packet: version=\(v)"
        case .invalidIHL(let ihl):
            return "Invalid IHL: \(ihl) (minimum is 5)"
        case .invalidDataOffset(let offset):
            return "Invalid TCP data offset: \(offset) (minimum is 5)"
        }
    }
}

/// Parses and builds IPv4 headers (minimum 20 bytes).
///
/// All multi-byte fields use network byte order (big-endian).
/// IP addresses are stored as Data(4).
struct IPv4Header {
    static let protocolTCP: UInt8 = 6
    static let protocolUDP: UInt8 = 17

    let version: UInt8
    let ihl: UInt8
    let tos: UInt8
    let totalLength: UInt16
    let identification: UInt16
    let flags: UInt8
    let fragmentOffset: UInt16
    let ttl: UInt8
    let `protocol`: UInt8
    let headerChecksum: UInt16
    let sourceAddress: Data
    let destAddress: Data

    var headerLength: Int {
        return Int(ihl) * 4
    }

    var sourceAddressBytes: Data {
        return Data(sourceAddress)
    }

    var destAddressBytes: Data {
        return Data(destAddress)
    }

    /// Serialize the header back to bytes, recalculating the checksum.
    func toBytes() -> Data {
        var bytes = Data(count: headerLength)

        // Byte 0: version (4 bits) + IHL (4 bits)
        bytes[0] = (version << 4) | ihl

        // Byte 1: TOS
        bytes[1] = tos

        // Bytes 2-3: Total length
        bytes[2] = UInt8(totalLength >> 8)
        bytes[3] = UInt8(totalLength & 0xFF)

        // Bytes 4-5: Identification
        bytes[4] = UInt8(identification >> 8)
        bytes[5] = UInt8(identification & 0xFF)

        // Bytes 6-7: Flags (3 bits) + Fragment offset (13 bits)
        let flagsAndOffset = (UInt16(flags) << 13) | fragmentOffset
        bytes[6] = UInt8(flagsAndOffset >> 8)
        bytes[7] = UInt8(flagsAndOffset & 0xFF)

        // Byte 8: TTL
        bytes[8] = ttl

        // Byte 9: Protocol
        bytes[9] = self.`protocol`

        // Bytes 10-11: Header checksum (set to 0 for calculation)
        bytes[10] = 0
        bytes[11] = 0

        // Bytes 12-15: Source address
        bytes.replaceSubrange(12..<16, with: sourceAddress)

        // Bytes 16-19: Destination address
        bytes.replaceSubrange(16..<20, with: destAddress)

        // Calculate and set checksum
        let checksum = Checksum.ipChecksum(data: bytes, offset: 0, length: headerLength)
        bytes[10] = UInt8(checksum >> 8)
        bytes[11] = UInt8(checksum & 0xFF)

        return bytes
    }

    /// Parse an IPv4 header from raw bytes.
    ///
    /// - Parameters:
    ///   - data: the data containing the IP packet
    ///   - offset: the starting position within the data
    /// - Returns: a parsed IPv4Header
    /// - Throws: PacketError if the buffer is too short or the version is not 4
    static func parse(data: Data, offset: Int = 0) throws -> IPv4Header {
        guard data.count - offset >= 20 else {
            throw PacketError.bufferTooShort(needed: 20, have: data.count - offset)
        }

        let versionIhl = data[offset]
        let version = versionIhl >> 4
        let ihl = versionIhl & 0x0F

        guard version == 4 else {
            throw PacketError.invalidVersion(version)
        }
        guard ihl >= 5 else {
            throw PacketError.invalidIHL(ihl)
        }
        guard data.count - offset >= Int(ihl) * 4 else {
            throw PacketError.bufferTooShort(needed: Int(ihl) * 4, have: data.count - offset)
        }

        let tos = data[offset + 1]

        let totalLength = (UInt16(data[offset + 2]) << 8) |
            UInt16(data[offset + 3])

        let identification = (UInt16(data[offset + 4]) << 8) |
            UInt16(data[offset + 5])

        let flagsAndOffset = (UInt16(data[offset + 6]) << 8) |
            UInt16(data[offset + 7])
        let flags = UInt8(flagsAndOffset >> 13)
        let fragmentOffset = flagsAndOffset & 0x1FFF

        let ttl = data[offset + 8]
        let proto = data[offset + 9]

        let headerChecksum = (UInt16(data[offset + 10]) << 8) |
            UInt16(data[offset + 11])

        let sourceAddress = data.subdata(in: (offset + 12)..<(offset + 16))
        let destAddress = data.subdata(in: (offset + 16)..<(offset + 20))

        return IPv4Header(
            version: version,
            ihl: ihl,
            tos: tos,
            totalLength: totalLength,
            identification: identification,
            flags: flags,
            fragmentOffset: fragmentOffset,
            ttl: ttl,
            protocol: proto,
            headerChecksum: headerChecksum,
            sourceAddress: sourceAddress,
            destAddress: destAddress
        )
    }
}
