import Foundation

/// Parses and builds TCP headers.
///
/// Sequence and acknowledgment numbers use UInt32 (Swift has proper unsigned types).
/// All multi-byte fields use network byte order (big-endian).
struct TcpHeader {
    static let flagFIN: UInt16 = 0x001
    static let flagSYN: UInt16 = 0x002
    static let flagRST: UInt16 = 0x004
    static let flagPSH: UInt16 = 0x008
    static let flagACK: UInt16 = 0x010
    static let flagURG: UInt16 = 0x020
    static let flagECE: UInt16 = 0x040
    static let flagCWR: UInt16 = 0x080
    static let flagNS:  UInt16 = 0x100

    let sourcePort: UInt16
    let destPort: UInt16
    let sequenceNumber: UInt32
    let ackNumber: UInt32
    let dataOffset: UInt8
    let reserved: UInt8
    let flags: UInt16
    let window: UInt16
    let checksum: UInt16
    let urgentPointer: UInt16

    var headerLength: Int {
        return Int(dataOffset) * 4
    }

    var isSyn: Bool { return (flags & TcpHeader.flagSYN) != 0 }
    var isAck: Bool { return (flags & TcpHeader.flagACK) != 0 }
    var isFin: Bool { return (flags & TcpHeader.flagFIN) != 0 }
    var isRst: Bool { return (flags & TcpHeader.flagRST) != 0 }
    var isPsh: Bool { return (flags & TcpHeader.flagPSH) != 0 }

    /// Serialize the TCP header to bytes.
    ///
    /// The checksum field is written as-is. Callers should compute the
    /// correct checksum using `Checksum.tcpUdpChecksum` after building the header.
    func toBytes() -> Data {
        var bytes = Data(count: headerLength)

        // Bytes 0-1: Source port
        bytes[0] = UInt8(sourcePort >> 8)
        bytes[1] = UInt8(sourcePort & 0xFF)

        // Bytes 2-3: Destination port
        bytes[2] = UInt8(destPort >> 8)
        bytes[3] = UInt8(destPort & 0xFF)

        // Bytes 4-7: Sequence number
        bytes[4] = UInt8((sequenceNumber >> 24) & 0xFF)
        bytes[5] = UInt8((sequenceNumber >> 16) & 0xFF)
        bytes[6] = UInt8((sequenceNumber >> 8) & 0xFF)
        bytes[7] = UInt8(sequenceNumber & 0xFF)

        // Bytes 8-11: Acknowledgment number
        bytes[8] = UInt8((ackNumber >> 24) & 0xFF)
        bytes[9] = UInt8((ackNumber >> 16) & 0xFF)
        bytes[10] = UInt8((ackNumber >> 8) & 0xFF)
        bytes[11] = UInt8(ackNumber & 0xFF)

        // Byte 12: Data offset (4 bits) + reserved (3 bits) + NS flag (1 bit)
        // Byte 13: Remaining 8 flag bits (CWR, ECE, URG, ACK, PSH, RST, SYN, FIN)
        let ns = UInt8((flags >> 8) & 0x01)
        bytes[12] = (dataOffset << 4) | (reserved << 1) | ns
        bytes[13] = UInt8(flags & 0xFF)

        // Bytes 14-15: Window size
        bytes[14] = UInt8(window >> 8)
        bytes[15] = UInt8(window & 0xFF)

        // Bytes 16-17: Checksum
        bytes[16] = UInt8(checksum >> 8)
        bytes[17] = UInt8(checksum & 0xFF)

        // Bytes 18-19: Urgent pointer
        bytes[18] = UInt8(urgentPointer >> 8)
        bytes[19] = UInt8(urgentPointer & 0xFF)

        return bytes
    }

    /// Parse a TCP header from raw bytes.
    ///
    /// - Parameters:
    ///   - data: the data containing the TCP segment
    ///   - offset: the starting position of the TCP header within the data
    /// - Returns: a parsed TcpHeader
    /// - Throws: PacketError if the buffer is too short
    static func parse(data: Data, offset: Int) throws -> TcpHeader {
        guard data.count - offset >= 20 else {
            throw PacketError.bufferTooShort(needed: 20, have: data.count - offset)
        }

        let sourcePort = (UInt16(data[offset]) << 8) |
            UInt16(data[offset + 1])

        let destPort = (UInt16(data[offset + 2]) << 8) |
            UInt16(data[offset + 3])

        let sequenceNumber = (UInt32(data[offset + 4]) << 24) |
            (UInt32(data[offset + 5]) << 16) |
            (UInt32(data[offset + 6]) << 8) |
            UInt32(data[offset + 7])

        let ackNumber = (UInt32(data[offset + 8]) << 24) |
            (UInt32(data[offset + 9]) << 16) |
            (UInt32(data[offset + 10]) << 8) |
            UInt32(data[offset + 11])

        let byte12 = data[offset + 12]
        let dataOffset = byte12 >> 4
        let reserved = (byte12 >> 1) & 0x07
        let ns = UInt16(byte12 & 0x01)

        let byte13 = UInt16(data[offset + 13])
        let flags = (ns << 8) | byte13

        let window = (UInt16(data[offset + 14]) << 8) |
            UInt16(data[offset + 15])

        let checksum = (UInt16(data[offset + 16]) << 8) |
            UInt16(data[offset + 17])

        let urgentPointer = (UInt16(data[offset + 18]) << 8) |
            UInt16(data[offset + 19])

        guard dataOffset >= 5 else {
            throw PacketError.invalidDataOffset(dataOffset)
        }

        return TcpHeader(
            sourcePort: sourcePort,
            destPort: destPort,
            sequenceNumber: sequenceNumber,
            ackNumber: ackNumber,
            dataOffset: dataOffset,
            reserved: reserved,
            flags: flags,
            window: window,
            checksum: checksum,
            urgentPointer: urgentPointer
        )
    }
}
