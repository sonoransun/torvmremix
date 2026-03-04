import Foundation

/// Error thrown when a SOCKS5 protocol error occurs.
enum Socks5Error: Error, LocalizedError {
    /// SOCKS5 protocol error with a human-readable description.
    case protocolError(String)

    /// SOCKS5 connect failed with a specific reply code from the server.
    case connectFailed(String, replyCode: Int)

    /// The connection to the SOCKS5 proxy failed or was interrupted.
    case connectionFailed(String, cause: Error?)

    /// Authentication with the SOCKS5 proxy failed.
    case authenticationFailed(String)

    var errorDescription: String? {
        switch self {
        case .protocolError(let message):
            return message
        case .connectFailed(let message, _):
            return message
        case .connectionFailed(let message, _):
            return message
        case .authenticationFailed(let message):
            return message
        }
    }

    var replyCode: Int {
        switch self {
        case .connectFailed(_, let code):
            return code
        default:
            return -1
        }
    }
}
