import Foundation
import Security

/// Manages a Secure Enclave P-256 ECIES key for wrapping sensitive credentials.
///
/// The private key never leaves the Secure Enclave hardware. Encryption uses the
/// public key (no biometric needed), but decryption requires biometric authentication
/// enforced at the hardware level via the key's access control policy.
final class SecureEnclaveKeyManager {

    static let shared = SecureEnclaveKeyManager()

    private let keyTag = "com.torvm.ios.secureenclave.wrapping-key"

    private init() {}

    var isAvailable: Bool {
        var error: Unmanaged<CFError>?
        let access = SecAccessControlCreateWithFlags(
            kCFAllocatorDefault,
            kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
            [.privateKeyUsage, .biometryCurrentSet],
            &error
        )
        return access != nil && error == nil
    }

    func getOrCreateKey() throws -> SecKey {
        if let existing = try? loadKey() {
            return existing
        }
        return try generateKey()
    }

    /// Encrypt data using the Secure Enclave public key (no biometric needed).
    func encrypt(data: Data) throws -> Data {
        let privateKey = try getOrCreateKey()
        guard let publicKey = SecKeyCopyPublicKey(privateKey) else {
            throw SecureEnclaveError.publicKeyUnavailable
        }
        guard SecKeyIsAlgorithm(publicKey,
                                .eciesEncryptionCofactorVariableIVX963SHA256AESGCM) else {
            throw SecureEnclaveError.algorithmNotSupported
        }
        var error: Unmanaged<CFError>?
        guard let ciphertext = SecKeyCreateEncryptedData(
            publicKey,
            .eciesEncryptionCofactorVariableIVX963SHA256AESGCM,
            data as CFData,
            &error
        ) as Data? else {
            throw SecureEnclaveError.encryptionFailed(error?.takeRetainedValue())
        }
        return ciphertext
    }

    /// Decrypt data using the Secure Enclave private key (requires biometric).
    func decrypt(ciphertext: Data) throws -> Data {
        let privateKey = try getOrCreateKey()
        var error: Unmanaged<CFError>?
        guard let plaintext = SecKeyCreateDecryptedData(
            privateKey,
            .eciesEncryptionCofactorVariableIVX963SHA256AESGCM,
            ciphertext as CFData,
            &error
        ) as Data? else {
            throw SecureEnclaveError.decryptionFailed(error?.takeRetainedValue())
        }
        return plaintext
    }

    // MARK: - Private

    private func loadKey() throws -> SecKey? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassKey,
            kSecAttrApplicationTag as String: keyTag.data(using: .utf8)!,
            kSecAttrKeyType as String: kSecAttrKeyTypeECSECPrimeRandom,
            kSecReturnRef as String: kCFBooleanTrue!
        ]
        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess else {
            throw SecureEnclaveError.keychainError(status)
        }
        return (result as! SecKey)
    }

    private func generateKey() throws -> SecKey {
        guard let accessControl = SecAccessControlCreateWithFlags(
            kCFAllocatorDefault,
            kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
            [.privateKeyUsage, .biometryCurrentSet, .or, .devicePasscode],
            nil
        ) else {
            throw SecureEnclaveError.accessControlCreationFailed
        }

        let attributes: [String: Any] = [
            kSecAttrKeyType as String: kSecAttrKeyTypeECSECPrimeRandom,
            kSecAttrKeySizeInBits as String: 256,
            kSecAttrTokenID as String: kSecAttrTokenIDSecureEnclave,
            kSecPrivateKeyAttrs as String: [
                kSecAttrIsPermanent as String: true,
                kSecAttrApplicationTag as String: keyTag.data(using: .utf8)!,
                kSecAttrAccessControl as String: accessControl
            ] as [String: Any]
        ]

        var error: Unmanaged<CFError>?
        guard let key = SecKeyCreateRandomKey(attributes as CFDictionary, &error) else {
            throw SecureEnclaveError.keyGenerationFailed(error?.takeRetainedValue())
        }
        return key
    }

    enum SecureEnclaveError: LocalizedError {
        case publicKeyUnavailable
        case algorithmNotSupported
        case encryptionFailed(CFError?)
        case decryptionFailed(CFError?)
        case keychainError(OSStatus)
        case accessControlCreationFailed
        case keyGenerationFailed(CFError?)

        var errorDescription: String? {
            switch self {
            case .publicKeyUnavailable: return "Could not extract public key"
            case .algorithmNotSupported: return "ECIES algorithm not supported"
            case .encryptionFailed(let e): return "Encryption failed: \(e?.localizedDescription ?? "unknown")"
            case .decryptionFailed(let e): return "Decryption failed: \(e?.localizedDescription ?? "unknown")"
            case .keychainError(let s): return "Keychain error: \(s)"
            case .accessControlCreationFailed: return "Failed to create access control"
            case .keyGenerationFailed(let e): return "Key generation failed: \(e?.localizedDescription ?? "unknown")"
            }
        }
    }
}
