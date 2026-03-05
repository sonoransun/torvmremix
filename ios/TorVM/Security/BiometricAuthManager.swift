import Foundation
import LocalAuthentication
import UIKit

enum BiometricType {
    case faceID
    case touchID
    case none
}

/// Manages biometric (Face ID / Touch ID) authentication gating for the app.
///
/// Configurable: VPN toggle and settings changes can independently require biometric.
/// Auth results are cached for a configurable duration to avoid repeated prompts.
/// Cache is cleared when the app goes to background.
@MainActor
final class BiometricAuthManager: ObservableObject {

    static let shared = BiometricAuthManager()

    private var backgroundObserver: NSObjectProtocol?

    @Published var cacheDuration: TimeInterval = 300
    @Published var requireBiometricForVPN: Bool = true
    @Published var requireBiometricForSettings: Bool = true

    var biometricType: BiometricType {
        let context = LAContext()
        var error: NSError?
        guard context.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics,
                                        error: &error) else {
            return .none
        }
        switch context.biometryType {
        case .faceID: return .faceID
        case .touchID: return .touchID
        case .opticID: return .faceID
        @unknown default: return .none
        }
    }

    var isAuthenticationAvailable: Bool {
        let context = LAContext()
        var error: NSError?
        return context.canEvaluatePolicy(.deviceOwnerAuthentication, error: &error)
    }

    // MARK: - Cached Auth

    private var lastAuthTime: Date?

    var isAuthCurrent: Bool {
        guard cacheDuration > 0, let lastAuth = lastAuthTime else { return false }
        return Date().timeIntervalSince(lastAuth) < cacheDuration
    }

    /// Authenticate the user with biometric or passcode. Returns true on success.
    func authenticate(reason: String) async -> Bool {
        if isAuthCurrent { return true }

        let context = LAContext()
        context.localizedCancelTitle = "Cancel"

        do {
            let success = try await context.evaluatePolicy(
                .deviceOwnerAuthentication,
                localizedReason: reason
            )
            if success {
                lastAuthTime = Date()
            }
            return success
        } catch {
            return false
        }
    }

    func clearCache() {
        lastAuthTime = nil
    }

    func registerForBackgroundNotification() {
        backgroundObserver = NotificationCenter.default.addObserver(
            forName: UIApplication.didEnterBackgroundNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            self?.clearCache()
        }
    }

    deinit {
        if let observer = backgroundObserver {
            NotificationCenter.default.removeObserver(observer)
        }
    }

    func authenticateForVPN() async -> Bool {
        guard requireBiometricForVPN else { return true }
        return await authenticate(reason: "Authenticate to toggle VPN connection")
    }

    func authenticateForSettings() async -> Bool {
        guard requireBiometricForSettings else { return true }
        return await authenticate(reason: "Authenticate to modify settings")
    }

    // MARK: - Persistence

    private let defaults = UserDefaults(suiteName: "group.com.torvm.shared")

    func loadPreferences() {
        requireBiometricForVPN = defaults?.bool(forKey: "bio_vpn") ?? true
        requireBiometricForSettings = defaults?.bool(forKey: "bio_settings") ?? true
        let cached = defaults?.double(forKey: "bio_cache") ?? 0
        cacheDuration = cached > 0 ? cached : 300
    }

    func savePreferences() {
        defaults?.set(requireBiometricForVPN, forKey: "bio_vpn")
        defaults?.set(requireBiometricForSettings, forKey: "bio_settings")
        defaults?.set(cacheDuration, forKey: "bio_cache")
    }
}
