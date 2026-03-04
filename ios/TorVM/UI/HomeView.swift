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

    var body: some View {
        NavigationStack {
            VStack(spacing: 24) {
                Spacer()

                StatusIndicator(state: vpnManager.connectionState)
                    .frame(width: 80, height: 80)

                Text(stateText)
                    .font(.title)

                if let error = vpnManager.errorMessage {
                    Text(error)
                        .foregroundStyle(.red)
                        .font(.callout)
                        .multilineTextAlignment(.center)
                        .padding(.horizontal)
                }

                if let config = try? KeychainManager.shared.loadConfig() {
                    ConnectionCard(config: config)
                        .padding(.horizontal)
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

                Spacer()

                NavigationLink(destination: LogView()) {
                    Text("View Logs")
                        .font(.callout)
                }
                .padding(.bottom)
            }
            .navigationTitle("TorVM")
            .toolbar {
                ToolbarItem(placement: .navigationBarTrailing) {
                    NavigationLink(destination: SettingsView()) {
                        Image(systemName: "gearshape")
                    }
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
