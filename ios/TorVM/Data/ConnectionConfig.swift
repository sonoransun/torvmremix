import Foundation

/// Connection configuration for the TorVM SOCKS5 proxy and DNS resolver.
/// Mirrors the Android ConnectionConfig.kt data class.
struct ConnectionConfig: Codable, Equatable {
    var socksHost: String
    var socksPort: UInt16
    var dnsHost: String
    var dnsPort: UInt16

    static let direct = ConnectionConfig(
        socksHost: "10.10.10.1",
        socksPort: 9050,
        dnsHost: "10.10.10.1",
        dnsPort: 9093
    )

    static let wifiDefault = ConnectionConfig(
        socksHost: "192.168.1.100",
        socksPort: 9050,
        dnsHost: "192.168.1.100",
        dnsPort: 9093
    )

    func validate() throws {
        guard !socksHost.isEmpty else { throw ConfigError.blankHost("socksHost") }
        guard !dnsHost.isEmpty else { throw ConfigError.blankHost("dnsHost") }
        guard (1...65535).contains(Int(socksPort)) else { throw ConfigError.invalidPort("socksPort") }
        guard (1...65535).contains(Int(dnsPort)) else { throw ConfigError.invalidPort("dnsPort") }
    }

    enum ConfigError: LocalizedError {
        case blankHost(String)
        case invalidPort(String)

        var errorDescription: String? {
            switch self {
            case .blankHost(let f): return "\(f) must not be blank"
            case .invalidPort(let f): return "\(f) must be 1-65535"
            }
        }
    }
}
