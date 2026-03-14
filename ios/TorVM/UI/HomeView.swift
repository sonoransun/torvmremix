import SwiftUI

struct HomeView: View {
    @StateObject private var vpnManager = VPNManager.shared
    @StateObject private var bioAuth = BiometricAuthManager.shared

    @State private var showAuthFailure = false

    private var stateText: String {
        switch vpnManager.connectionState {
        case .connected: return "Connected"
        case .connecting: return "Connecting..."
        case .disconnected: return "Disconnected"
        case .disconnecting: return "Disconnecting..."
        case .error: return "Error"
        }
    }

    private var isActive: Bool {
        vpnManager.connectionState == .connected || vpnManager.connectionState == .connecting
    }

    private var statusAccessibilityLabel: String {
        switch vpnManager.connectionState {
        case .connected: return "VPN Status: Connected"
        case .connecting: return "VPN Status: Connecting"
        case .disconnected: return "VPN Status: Disconnected"
        case .disconnecting: return "VPN Status: Disconnecting"
        case .error: return "VPN Status: Error"
        }
    }

    private var statusAccessibilityHint: String {
        switch vpnManager.connectionState {
        case .connected: return "Double tap the connect button to disconnect."
        case .connecting: return "Currently establishing connection."
        case .disconnected: return "Double tap the connect button to connect."
        case .disconnecting: return "Currently disconnecting."
        case .error: return "An error occurred. Double tap the connect button to retry."
        }
    }

    var body: some View {
        NavigationStack {
            VStack(spacing: 24) {
                Spacer()

                StatusIndicator(state: vpnManager.connectionState)
                    .frame(width: 80, height: 80)
                    .accessibilityLabel(statusAccessibilityLabel)
                    .accessibilityHint(statusAccessibilityHint)

                Text(stateText)
                    .font(.title)
                    .accessibilityHidden(true)

                if let error = vpnManager.errorMessage {
                    Text(error)
                        .foregroundStyle(.red)
                        .font(.callout)
                        .multilineTextAlignment(.center)
                        .padding(.horizontal)
                        .accessibilityLabel("Error: \(error)")
                }

                if let config = try? KeychainManager.shared.loadConfig() {
                    ConnectionCard(config: config)
                        .padding(.horizontal)
                        .accessibilityLabel("Connection configuration. SOCKS5: \(config.socksHost) port \(config.socksPort). DNS: \(config.dnsHost) port \(config.dnsPort).")
                }

                Button {
                    Task {
                        let authed = await bioAuth.authenticateForVPN()
                        if authed {
                            await vpnManager.toggle()
                        } else {
                            showAuthFailure = true
                        }
                    }
                } label: {
                    Text(isActive ? "Disconnect" : "Connect")
                        .font(.headline)
                        .frame(maxWidth: .infinity)
                        .frame(height: 50)
                }
                .buttonStyle(.borderedProminent)
                .tint(isActive ? .red : .purple)
                .padding(.horizontal)
                .disabled(vpnManager.connectionState == .disconnecting)
                .accessibilityLabel(isActive ? "Disconnect from TorVM" : "Connect to TorVM")
                .accessibilityHint(isActive ? "Double tap to disconnect the VPN." : "Double tap to connect the VPN.")

                Spacer()

                NavigationLink(destination: LogView()) {
                    Text("View Logs")
                        .font(.callout)
                }
                .padding(.bottom)
                .accessibilityLabel("View Logs")
                .accessibilityHint("Double tap to open the log viewer.")
            }
            .navigationTitle("TorVM")
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    NavigationLink(destination: SettingsView()) {
                        Image(systemName: "gearshape")
                    }
                    .accessibilityLabel("Settings")
                    .accessibilityHint("Double tap to open settings.")
                }
            }
            .alert("Authentication Failed",
                   isPresented: $showAuthFailure) {
                Button("OK", role: .cancel) {}
            } message: {
                Text("Biometric or passcode authentication is required to toggle the VPN.")
            }
        }
    }
}

#Preview {
    HomeView()
}
