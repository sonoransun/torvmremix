import Foundation
import Network

/// SOCKS5 CONNECT client per RFC 1928.
///
/// Establishes a connection through a SOCKS5 proxy server (e.g., Tor's SOCKS port)
/// to a specified destination. Supports IPv4 address connections and hostname
/// connections. Uses `NWConnection` from the Network framework.
///
/// The client supports optional RFC 1929 username/password authentication.
/// Tor uses distinct SOCKS5 credentials to create separate circuits, enabling
/// per-app traffic isolation.
final class Socks5Client {
    private let proxyHost: String
    private let proxyPort: UInt16

    /// Optional SOCKS5 username/password authentication credentials.
    /// When set, the client offers both no-auth (0x00) and username/password (0x02)
    /// methods. Tor uses different SOCKS5 credentials to create isolated circuits.
    var authUsername: String?
    var authPassword: String?

    private static let socksVersion: UInt8 = 0x05
    private static let authNone: UInt8 = 0x00
    private static let authUserPass: UInt8 = 0x02
    private static let userPassVersion: UInt8 = 0x01
    private static let cmdConnect: UInt8 = 0x01
    private static let reserved: UInt8 = 0x00
    private static let addrTypeIPv4: UInt8 = 0x01
    private static let addrTypeDomain: UInt8 = 0x03

    private static let connectTimeoutSeconds: TimeInterval = 10
    private static let socketTimeoutSeconds: TimeInterval = 120

    /// Create a new SOCKS5 client.
    ///
    /// - Parameters:
    ///   - proxyHost: the SOCKS5 proxy hostname or IP address
    ///   - proxyPort: the SOCKS5 proxy port
    init(proxyHost: String, proxyPort: UInt16) {
        self.proxyHost = proxyHost
        self.proxyPort = proxyPort
    }

    /// Connect to a destination specified by IPv4 address through the SOCKS5 proxy.
    ///
    /// - Parameters:
    ///   - destAddr: the destination IPv4 address (4 bytes)
    ///   - destPort: the destination port
    /// - Returns: a connected `NWConnection` ready for data transfer; the caller owns this connection
    /// - Throws: `Socks5Error` on SOCKS5 protocol errors or connection failures
    func connect(destAddr: Data, destPort: UInt16) async throws -> NWConnection {
        precondition(destAddr.count == 4, "destAddr must be 4 bytes for IPv4")

        let connection = try await createConnection()
        do {
            try await performGreeting(connection: connection)
            try await sendIpv4ConnectRequest(connection: connection, destAddr: destAddr, destPort: destPort)
            try await readConnectReply(connection: connection)
            return connection
        } catch {
            connection.cancel()
            throw error
        }
    }

    /// Connect to a destination specified by hostname through the SOCKS5 proxy.
    ///
    /// - Parameters:
    ///   - destHost: the destination hostname
    ///   - destPort: the destination port
    /// - Returns: a connected `NWConnection` ready for data transfer; the caller owns this connection
    /// - Throws: `Socks5Error` on SOCKS5 protocol errors or connection failures
    func connect(destHost: String, destPort: UInt16) async throws -> NWConnection {
        let connection = try await createConnection()
        do {
            try await performGreeting(connection: connection)
            try await sendDomainConnectRequest(connection: connection, destHost: destHost, destPort: destPort)
            try await readConnectReply(connection: connection)
            return connection
        } catch {
            connection.cancel()
            throw error
        }
    }

    // MARK: - Connection Setup

    /// Create and wait for an NWConnection to the SOCKS5 proxy.
    private func createConnection() async throws -> NWConnection {
        let host = NWEndpoint.Host(proxyHost)
        let port = NWEndpoint.Port(rawValue: proxyPort)!
        let params = NWParameters.tcp
        let connection = NWConnection(host: host, port: port, using: params)
        let timeout = Self.connectTimeoutSeconds

        return try await withCheckedThrowingContinuation { continuation in
            let once = ContinuationOnce(continuation: continuation)

            connection.stateUpdateHandler = { [once] state in
                switch state {
                case .ready:
                    connection.stateUpdateHandler = nil
                    once.resume(returning: connection)
                case .failed(let error):
                    connection.stateUpdateHandler = nil
                    once.resume(throwing: Socks5Error.connectionFailed(
                        "SOCKS5 proxy connection failed: \(error.localizedDescription)",
                        cause: error
                    ))
                case .cancelled:
                    connection.stateUpdateHandler = nil
                    once.resume(throwing: Socks5Error.connectionFailed(
                        "SOCKS5 proxy connection cancelled", cause: nil
                    ))
                default:
                    break
                }
            }
            connection.start(queue: .global(qos: .userInitiated))

            DispatchQueue.global().asyncAfter(deadline: .now() + timeout) { [once] in
                connection.stateUpdateHandler = nil
                connection.cancel()
                once.resume(throwing: Socks5Error.connectionFailed(
                    "SOCKS5 proxy connection timed out", cause: nil
                ))
            }
        }
    }

    // MARK: - SOCKS5 Greeting / Authentication

    /// Perform the SOCKS5 greeting/authentication negotiation.
    ///
    /// When `authUsername` and `authPassword` are set, offers both no-auth (0x00)
    /// and username/password (0x02) methods. If the server selects username/password,
    /// performs the sub-negotiation per RFC 1929.
    private func performGreeting(connection: NWConnection) async throws {
        let hasAuth = authUsername != nil && authPassword != nil

        var greeting: Data
        if hasAuth {
            // Offer 2 methods: no-auth and username/password
            greeting = Data([Self.socksVersion, 0x02, Self.authNone, Self.authUserPass])
        } else {
            // Offer 1 method: no-auth only
            greeting = Data([Self.socksVersion, 0x01, Self.authNone])
        }
        try await send(connection: connection, data: greeting)

        let response = try await receive(connection: connection, count: 2)

        guard response[0] == Self.socksVersion else {
            throw Socks5Error.protocolError(
                "Unexpected SOCKS version in greeting response: \(response[0])"
            )
        }

        switch response[1] {
        case Self.authNone:
            // No authentication required -- proceed
            break
        case Self.authUserPass:
            guard hasAuth else {
                throw Socks5Error.authenticationFailed(
                    "Server selected username/password auth but no credentials provided"
                )
            }
            try await performUsernamePasswordAuth(
                connection: connection,
                username: authUsername!,
                password: authPassword!
            )
        default:
            throw Socks5Error.protocolError(
                "SOCKS5 server selected unsupported auth method: \(response[1])"
            )
        }
    }

    /// Perform SOCKS5 username/password sub-negotiation per RFC 1929.
    ///
    /// Format: [0x01, ulen, username, plen, password]
    /// Response: [0x01, status] where 0x00 = success
    private func performUsernamePasswordAuth(
        connection: NWConnection,
        username: String,
        password: String
    ) async throws {
        let userBytes = Array(username.utf8)
        let passBytes = Array(password.utf8)
        precondition(userBytes.count <= 255, "Username too long: \(userBytes.count) bytes (max 255)")
        precondition(passBytes.count <= 255, "Password too long: \(passBytes.count) bytes (max 255)")

        var request = Data(capacity: 3 + userBytes.count + passBytes.count)
        request.append(Self.userPassVersion)
        request.append(UInt8(userBytes.count))
        request.append(contentsOf: userBytes)
        request.append(UInt8(passBytes.count))
        request.append(contentsOf: passBytes)

        try await send(connection: connection, data: request)

        let response = try await receive(connection: connection, count: 2)
        guard response[1] == 0x00 else {
            throw Socks5Error.authenticationFailed(
                "SOCKS5 username/password authentication failed: status \(response[1])"
            )
        }
    }

    // MARK: - SOCKS5 CONNECT Requests

    /// Send a SOCKS5 CONNECT request for an IPv4 address.
    ///
    /// Format: [0x05, 0x01, 0x00, 0x01, <4-byte addr>, <2-byte port BE>]
    private func sendIpv4ConnectRequest(
        connection: NWConnection,
        destAddr: Data,
        destPort: UInt16
    ) async throws {
        var request = Data(count: 10)
        request[0] = Self.socksVersion
        request[1] = Self.cmdConnect
        request[2] = Self.reserved
        request[3] = Self.addrTypeIPv4
        request.replaceSubrange(4..<8, with: destAddr)
        request[8] = UInt8(destPort >> 8)
        request[9] = UInt8(destPort & 0xFF)

        try await send(connection: connection, data: request)
    }

    /// Send a SOCKS5 CONNECT request for a domain name.
    ///
    /// Format: [0x05, 0x01, 0x00, 0x03, <len>, <domain bytes>, <2-byte port BE>]
    private func sendDomainConnectRequest(
        connection: NWConnection,
        destHost: String,
        destPort: UInt16
    ) async throws {
        let hostBytes = Array(destHost.utf8)
        precondition(hostBytes.count <= 255, "Domain name too long: \(hostBytes.count) bytes (max 255)")

        var request = Data(capacity: 4 + 1 + hostBytes.count + 2)
        request.append(Self.socksVersion)
        request.append(Self.cmdConnect)
        request.append(Self.reserved)
        request.append(Self.addrTypeDomain)
        request.append(UInt8(hostBytes.count))
        request.append(contentsOf: hostBytes)
        request.append(UInt8(destPort >> 8))
        request.append(UInt8(destPort & 0xFF))

        try await send(connection: connection, data: request)
    }

    // MARK: - SOCKS5 Reply

    /// Read and validate the SOCKS5 CONNECT reply.
    ///
    /// The minimum reply for IPv4 is 10 bytes:
    /// [version, reply, reserved, addr_type, 4-byte addr, 2-byte port]
    ///
    /// For domain replies the length varies. We handle IPv4 (0x01), domain (0x03),
    /// and IPv6 (0x04) address types to consume the full reply from the stream.
    private func readConnectReply(connection: NWConnection) async throws {
        // Read the first 4 bytes: version, reply, reserved, address type
        let header = try await receive(connection: connection, count: 4)

        guard header[0] == Self.socksVersion else {
            throw Socks5Error.protocolError(
                "Unexpected SOCKS version in connect reply: \(header[0])"
            )
        }

        let replyCode = Int(header[1])
        guard replyCode == 0x00 else {
            throw Socks5Error.connectFailed(
                "SOCKS5 connect failed: reply code 0x\(String(replyCode, radix: 16, uppercase: false).leftPadded(toLength: 2, with: "0"))",
                replyCode: replyCode
            )
        }

        // Consume the bound address and port based on address type
        let addrType = Int(header[3])
        switch addrType {
        case 0x01:
            // IPv4: 4 bytes address + 2 bytes port
            _ = try await receive(connection: connection, count: 6)
        case 0x03:
            // Domain: 1 byte length + N bytes domain + 2 bytes port
            let lenBuf = try await receive(connection: connection, count: 1)
            let domainLen = Int(lenBuf[0])
            _ = try await receive(connection: connection, count: domainLen + 2)
        case 0x04:
            // IPv6: 16 bytes address + 2 bytes port
            _ = try await receive(connection: connection, count: 18)
        default:
            throw Socks5Error.protocolError(
                "Unknown SOCKS5 address type in reply: 0x\(String(addrType, radix: 16))"
            )
        }
    }

    // MARK: - NWConnection Helpers

    /// Send data over the NWConnection.
    private func send(connection: NWConnection, data: Data) async throws {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            connection.send(content: data, completion: .contentProcessed { error in
                if let error = error {
                    continuation.resume(throwing: Socks5Error.connectionFailed(
                        "SOCKS5 send failed: \(error.localizedDescription)", cause: error
                    ))
                } else {
                    continuation.resume()
                }
            })
        }
    }

    /// Receive exactly `count` bytes from the NWConnection.
    private func receive(connection: NWConnection, count: Int) async throws -> Data {
        try await withCheckedThrowingContinuation { continuation in
            connection.receive(minimumIncompleteLength: count, maximumLength: count) { content, _, _, error in
                if let error = error {
                    continuation.resume(throwing: Socks5Error.connectionFailed(
                        "SOCKS5 receive failed: \(error.localizedDescription)", cause: error
                    ))
                } else if let data = content, data.count == count {
                    continuation.resume(returning: data)
                } else {
                    let have = content?.count ?? 0
                    continuation.resume(throwing: Socks5Error.connectionFailed(
                        "SOCKS5 connection closed unexpectedly: read \(have) of \(count) bytes",
                        cause: nil
                    ))
                }
            }
        }
    }
}

// MARK: - ContinuationOnce

/// Thread-safe wrapper ensuring a CheckedContinuation is resumed exactly once.
private final class ContinuationOnce<T>: @unchecked Sendable {
    private var continuation: CheckedContinuation<T, Error>?
    private let lock = NSLock()

    init(continuation: CheckedContinuation<T, Error>) {
        self.continuation = continuation
    }

    func resume(returning value: T) {
        lock.lock()
        let c = continuation
        continuation = nil
        lock.unlock()
        c?.resume(returning: value)
    }

    func resume(throwing error: Error) {
        lock.lock()
        let c = continuation
        continuation = nil
        lock.unlock()
        c?.resume(throwing: error)
    }
}

// MARK: - String Helpers

private extension String {
    func leftPadded(toLength length: Int, with character: Character) -> String {
        if count >= length { return self }
        return String(repeating: character, count: length - count) + self
    }
}
