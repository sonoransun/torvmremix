import Foundation
import NetworkExtension
import Combine

enum VPNConnectionState: String, CaseIterable {
    case disconnected
    case connecting
    case connected
    case disconnecting
    case error

    init(from neStatus: NEVPNStatus) {
        switch neStatus {
        case .disconnected, .invalid: self = .disconnected
        case .connecting, .reasserting: self = .connecting
        case .connected: self = .connected
        case .disconnecting: self = .disconnecting
        @unknown default: self = .disconnected
        }
    }
}

/// Manages the NETunnelProviderManager lifecycle for the TorVM VPN.
///
/// Installs or loads the VPN configuration, starts/stops the tunnel,
/// and observes connection state changes via NEVPNStatusDidChange.
@MainActor
final class VPNManager: ObservableObject {

    static let shared = VPNManager()

    @Published private(set) var connectionState: VPNConnectionState = .disconnected
    @Published private(set) var errorMessage: String?
    @Published private(set) var connectedDate: Date?

    private var manager: NETunnelProviderManager?
    private var statusObserver: AnyCancellable?

    private init() {
        Task { await loadOrCreateManager() }
    }

    func loadOrCreateManager() async {
        do {
            let managers = try await NETunnelProviderManager.loadAllFromPreferences()
            if let existing = managers.first {
                manager = existing
            } else {
                let newManager = NETunnelProviderManager()
                let proto = NETunnelProviderProtocol()
                proto.providerBundleIdentifier = "com.torvm.ios.tunnel"
                proto.serverAddress = "TorVM"
                proto.disconnectOnSleep = false
                newManager.protocolConfiguration = proto
                newManager.localizedDescription = "TorVM"
                newManager.isEnabled = true
                try await newManager.saveToPreferences()
                try await newManager.loadFromPreferences()
                manager = newManager
            }
            observeStatus()
        } catch {
            errorMessage = "Failed to load VPN config: \(error.localizedDescription)"
        }
    }

    func startTunnel() async {
        guard let manager = manager else {
            errorMessage = "VPN manager not initialized"
            return
        }
        do {
            let config = try KeychainManager.shared.loadConfig()
            let configData = try JSONEncoder().encode(config)
            let options: [String: NSObject] = [
                "config": configData as NSData
            ]
            try manager.connection.startVPNTunnel(options: options)
        } catch {
            errorMessage = "Failed to start VPN: \(error.localizedDescription)"
            connectionState = .error
        }
    }

    func stopTunnel() {
        manager?.connection.stopVPNTunnel()
    }

    func toggle() async {
        switch connectionState {
        case .connected, .connecting:
            stopTunnel()
        case .disconnected, .error:
            await startTunnel()
        case .disconnecting:
            break
        }
    }

    // MARK: - Status Observation

    private func observeStatus() {
        statusObserver = NotificationCenter.default
            .publisher(for: .NEVPNStatusDidChange, object: manager?.connection)
            .receive(on: DispatchQueue.main)
            .sink { [weak self] _ in
                guard let self = self, let conn = self.manager?.connection else { return }
                let newState = VPNConnectionState(from: conn.status)
                self.connectionState = newState
                if newState == .connected {
                    self.connectedDate = conn.connectedDate
                    self.errorMessage = nil
                } else if newState == .disconnected {
                    self.connectedDate = nil
                }
            }

        if let conn = manager?.connection {
            connectionState = VPNConnectionState(from: conn.status)
        }
    }
}
