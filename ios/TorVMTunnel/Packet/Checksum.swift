import Foundation

/// Utility for computing IP and TCP/UDP checksums per RFC 791.
///
/// All checksums use the standard one's complement sum algorithm:
/// sum all 16-bit words, fold any carries back in, then take the one's complement.
enum Checksum {

    /// Compute the standard RFC 791 IP checksum over the given byte range.
    ///
    /// The algorithm:
    /// 1. Sum all 16-bit words in the data.
    /// 2. If there is an odd byte at the end, pad it with a zero byte and add.
    /// 3. Fold any carry bits (upper 16 bits) back into the lower 16 bits.
    /// 4. Take the one's complement of the result.
    ///
    /// - Parameters:
    ///   - data: the data to checksum
    ///   - offset: starting position within the data
    ///   - length: number of bytes to include
    /// - Returns: the 16-bit checksum value
    static func ipChecksum(data: Data, offset: Int = 0, length: Int? = nil) -> UInt16 {
        let len = length ?? data.count
        var sum: UInt32 = 0
        var i = 0

        // Sum 16-bit words
        while i + 1 < len {
            let word = (UInt32(data[offset + i]) << 8) |
                UInt32(data[offset + i + 1])
            sum += word
            i += 2
        }

        // If odd number of bytes, pad the last byte with zero
        if i < len {
            sum += UInt32(data[offset + i]) << 8
        }

        // Fold 32-bit sum into 16 bits
        while (sum >> 16) != 0 {
            sum = (sum & 0xFFFF) + (sum >> 16)
        }

        // One's complement
        return UInt16(~sum & 0xFFFF)
    }

    /// Verify a TCP or UDP checksum by computing the checksum over the
    /// pseudo-header plus the raw transport segment (including the stored
    /// checksum field). Returns true if the checksum is valid.
    ///
    /// - Parameters:
    ///   - ipHeader: the IPv4 header (provides source/dest addresses and protocol)
    ///   - packet: the raw packet data
    ///   - transportOffset: byte offset where the transport segment begins
    ///   - length: total valid bytes in the packet
    /// - Returns: true if the checksum is valid
    static func verifyTransportChecksum(
        ipHeader: IPv4Header,
        packet: Data,
        transportOffset: Int,
        length: Int
    ) -> Bool {
        let segmentLength = length - transportOffset
        if segmentLength < 8 { return false } // Minimum UDP header size

        let totalLength = 12 + segmentLength
        var buffer = Data(count: totalLength)

        // Pseudo-header: source address (4 bytes)
        buffer.replaceSubrange(0..<4, with: ipHeader.sourceAddress)
        // Pseudo-header: destination address (4 bytes)
        buffer.replaceSubrange(4..<8, with: ipHeader.destAddress)
        // Pseudo-header: zero + protocol + segment length
        buffer[8] = 0
        buffer[9] = ipHeader.`protocol`
        buffer[10] = UInt8(segmentLength >> 8)
        buffer[11] = UInt8(segmentLength & 0xFF)

        // Transport segment (including stored checksum field)
        buffer.replaceSubrange(12..<(12 + segmentLength),
                               with: packet.subdata(in: transportOffset..<(transportOffset + segmentLength)))

        // If checksum is valid, ipChecksum over pseudo-header + segment = 0
        return ipChecksum(data: buffer) == 0
    }

    /// Compute the TCP or UDP checksum using the IPv4 pseudo-header.
    ///
    /// The pseudo-header consists of:
    /// - Source address (4 bytes)
    /// - Destination address (4 bytes)
    /// - Zero byte (1 byte)
    /// - Protocol number (1 byte)
    /// - TCP/UDP segment length (2 bytes, big-endian)
    ///
    /// This is concatenated with the transport header and payload, then the
    /// standard one's complement checksum is computed over the whole thing.
    ///
    /// - Parameters:
    ///   - ipHeader: the IPv4 header (provides source/dest addresses and protocol)
    ///   - transportHeader: the TCP or UDP header bytes (with checksum field zeroed)
    ///   - payload: the data payload following the transport header
    /// - Returns: the 16-bit checksum value
    static func tcpUdpChecksum(
        ipHeader: IPv4Header,
        transportHeader: Data,
        payload: Data = Data()
    ) -> UInt16 {
        let segmentLength = transportHeader.count + payload.count

        // Build pseudo-header (12 bytes) + transport header + payload
        let totalLength = 12 + segmentLength
        var buffer = Data(count: totalLength)

        // Pseudo-header: source address (4 bytes)
        buffer.replaceSubrange(0..<4, with: ipHeader.sourceAddress)

        // Pseudo-header: destination address (4 bytes)
        buffer.replaceSubrange(4..<8, with: ipHeader.destAddress)

        // Pseudo-header: zero + protocol + segment length
        buffer[8] = 0
        buffer[9] = ipHeader.`protocol`
        buffer[10] = UInt8(segmentLength >> 8)
        buffer[11] = UInt8(segmentLength & 0xFF)

        // Transport header
        buffer.replaceSubrange(12..<(12 + transportHeader.count), with: transportHeader)

        // Payload
        if !payload.isEmpty {
            buffer.replaceSubrange((12 + transportHeader.count)..<(12 + segmentLength), with: payload)
        }

        return ipChecksum(data: buffer)
    }
}
