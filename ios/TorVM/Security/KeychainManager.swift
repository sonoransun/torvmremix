import Foundation
import Security
import LocalAuthentication

enum KeychainError: LocalizedError {
    case duplicateItem
    case itemNotFound
    case unexpectedStatus(OSStatus)
    case encodingFailed
    case decodingFailed
    case accessControlCreationFailed

    var errorDescription: String? {
        switch self {
        case .duplicateItem: return "Keychain item already exists"
        case .itemNotFound: return "Keychain item not found"
        case .unexpectedStatus(let s): return "Keychain error: \(s)"
        case .encodingFailed: return "Failed to encode data"
        case .decodingFailed: return "Failed to decode data"
        case .accessControlCreationFailed: return "Failed to create access control"
        }
    }
}

/// Manages all Keychain operations for TorVM.
///
/// Uses App Group Keychain sharing so both the main app and the Network Extension
/// can access the same items. Config items use AfterFirstUnlock accessibility so
/// the extension can read them even when the device is locked. Credential items
/// use biometric-protected SecAccessControl for OS-enforced authentication.
final class KeychainManager {

    static let shared = KeychainManager()

    private let accessGroup = "com.torvm.shared"
    private let service = "com.torvm.ios"

    private init() {}

    // MARK: - ConnectionConfig (accessible by extension without biometric)

    func saveConfig(_ config: ConnectionConfig) throws {
        guard let data = try? JSONEncoder().encode(config) else {
            throw KeychainError.encodingFailed
        }

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: "connection_config",
            kSecAttrAccessGroup as String: accessGroup,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
            kSecAttrSynchronizable as String: kCFBooleanFalse!
        ]

        let status = SecItemAdd(query as CFDictionary, nil)
        if status == errSecDuplicateItem {
            let searchQuery: [String: Any] = [
                kSecClass as String: kSecClassGenericPassword,
                kSecAttrService as String: service,
                kSecAttrAccount as String: "connection_config",
                kSecAttrAccessGroup as String: accessGroup
            ]
            let updateAttributes: [String: Any] = [
                kSecValueData as String: data
            ]
            let updateStatus = SecItemUpdate(searchQuery as CFDictionary,
                                             updateAttributes as CFDictionary)
            guard updateStatus == errSecSuccess else {
                throw KeychainError.unexpectedStatus(updateStatus)
            }
        } else if status != errSecSuccess {
            throw KeychainError.unexpectedStatus(status)
        }
    }

    func loadConfig() throws -> ConnectionConfig {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: "connection_config",
            kSecAttrAccessGroup as String: accessGroup,
            kSecReturnData as String: kCFBooleanTrue!,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]

        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)

        guard status == errSecSuccess, let data = result as? Data else {
            if status == errSecItemNotFound {
                throw KeychainError.itemNotFound
            }
            throw KeychainError.unexpectedStatus(status)
        }

        guard let config = try? JSONDecoder().decode(ConnectionConfig.self, from: data) else {
            throw KeychainError.decodingFailed
        }
        return config
    }

    func configExists() -> Bool {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: "connection_config",
            kSecAttrAccessGroup as String: accessGroup,
            kSecReturnData as String: kCFBooleanFalse!
        ]
        return SecItemCopyMatching(query as CFDictionary, nil) == errSecSuccess
    }

    func deleteConfig() throws {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: "connection_config",
            kSecAttrAccessGroup as String: accessGroup
        ]
        let status = SecItemDelete(query as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw KeychainError.unexpectedStatus(status)
        }
    }

    // MARK: - Biometric-Protected Items

    func saveBiometricProtectedItem(account: String, data: Data) throws {
        guard let accessControl = SecAccessControlCreateWithFlags(
            kCFAllocatorDefault,
            kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
            [.biometryCurrentSet, .or, .devicePasscode],
            nil
        ) else {
            throw KeychainError.accessControlCreationFailed
        }

        // Delete existing before re-adding (ACL change requires re-creation).
        try? deleteItem(account: account)

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecAttrAccessGroup as String: accessGroup,
            kSecValueData as String: data,
            kSecAttrAccessControl as String: accessControl,
            kSecAttrSynchronizable as String: kCFBooleanFalse!
        ]

        let status = SecItemAdd(query as CFDictionary, nil)
        guard status == errSecSuccess else {
            throw KeychainError.unexpectedStatus(status)
        }
    }

    /// Load a biometric-protected item. Triggers Face ID / Touch ID prompt.
    func loadBiometricProtectedItem(account: String, promptReason: String) throws -> Data {
        let context = LAContext()
        context.localizedReason = promptReason

        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecAttrAccessGroup as String: accessGroup,
            kSecReturnData as String: kCFBooleanTrue!,
            kSecMatchLimit as String: kSecMatchLimitOne,
            kSecUseAuthenticationContext as String: context
        ]

        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        guard status == errSecSuccess, let data = result as? Data else {
            if status == errSecItemNotFound { throw KeychainError.itemNotFound }
            throw KeychainError.unexpectedStatus(status)
        }
        return data
    }

    func deleteItem(account: String) throws {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecAttrAccessGroup as String: accessGroup
        ]
        let status = SecItemDelete(query as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw KeychainError.unexpectedStatus(status)
        }
    }

    func itemExists(account: String) -> Bool {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecAttrAccessGroup as String: accessGroup,
            kSecReturnData as String: kCFBooleanFalse!
        ]
        return SecItemCopyMatching(query as CFDictionary, nil) == errSecSuccess
    }
}
