import SwiftUI

struct SettingsView: View {
    @StateObject private var bioAuth = BiometricAuthManager.shared

    @State private var socksHost = ""
    @State private var socksPort = ""
    @State private var dnsHost = ""
    @State private var dnsPort = ""
    @State private var showSaveError = false
    @State private var saveErrorMessage = ""
    @State private var showAuthFailure = false

    private var biometricLabel: String {
        switch bioAuth.biometricType {
        case .faceID: return "Face ID"
        case .touchID: return "Touch ID"
        case .none: return "Not Available"
        }
    }

    private var cacheDurationBinding: Binding<TimeInterval> {
        Binding(
            get: { bioAuth.cacheDuration },
            set: {
                bioAuth.cacheDuration = $0
                bioAuth.savePreferences()
            }
        )
    }

    var body: some View {
        Form {
            Section("Presets") {
                HStack(spacing: 12) {
                    Button("Direct (TAP)") {
                        applyPreset(.direct)
                    }
                    .buttonStyle(.bordered)
                    Button("WiFi/LAN") {
                        applyPreset(.wifiDefault)
                    }
                    .buttonStyle(.bordered)
                }
            }

            Section("Connection") {
                TextField("SOCKS5 Host", text: $socksHost)
                    .textContentType(.URL)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                TextField("SOCKS5 Port", text: $socksPort)
                    .keyboardType(.numberPad)
                TextField("DNS Host", text: $dnsHost)
                    .textContentType(.URL)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                TextField("DNS Port", text: $dnsPort)
                    .keyboardType(.numberPad)
            }

            Section("Security") {
                Toggle("Require biometric for VPN", isOn: Binding(
                    get: { bioAuth.requireBiometricForVPN },
                    set: {
                        bioAuth.requireBiometricForVPN = $0
                        bioAuth.savePreferences()
                    }
                ))
                Toggle("Require biometric for settings", isOn: Binding(
                    get: { bioAuth.requireBiometricForSettings },
                    set: {
                        bioAuth.requireBiometricForSettings = $0
                        bioAuth.savePreferences()
                    }
                ))
                Picker("Cache duration", selection: cacheDurationBinding) {
                    Text("1 minute").tag(TimeInterval(60))
                    Text("5 minutes").tag(TimeInterval(300))
                    Text("15 minutes").tag(TimeInterval(900))
                    Text("Never").tag(TimeInterval(0))
                }
                HStack {
                    Text("Biometric type")
                    Spacer()
                    Text(biometricLabel)
                        .foregroundStyle(.secondary)
                }
            }

            Section("Firewall") {
                Text("Per-app VPN routing is not available on iOS. All device traffic is routed through the TorVM tunnel when connected.")
                    .font(.callout)
                    .foregroundStyle(.secondary)
            }

            Section {
                Button {
                    Task {
                        let authed = await bioAuth.authenticateForSettings()
                        if authed {
                            saveConfig()
                        } else {
                            showAuthFailure = true
                        }
                    }
                } label: {
                    Text("Save")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.borderedProminent)
            }
        }
        .navigationTitle("Settings")
        .onAppear(perform: loadConfig)
        .alert("Save Error", isPresented: $showSaveError) {
            Button("OK", role: .cancel) {}
        } message: {
            Text(saveErrorMessage)
        }
        .alert("Authentication Failed", isPresented: $showAuthFailure) {
            Button("OK", role: .cancel) {}
        } message: {
            Text("Biometric or passcode authentication is required to save settings.")
        }
    }

    private func applyPreset(_ preset: ConnectionConfig) {
        socksHost = preset.socksHost
        socksPort = String(preset.socksPort)
        dnsHost = preset.dnsHost
        dnsPort = String(preset.dnsPort)
    }

    private func loadConfig() {
        if let config = try? KeychainManager.shared.loadConfig() {
            socksHost = config.socksHost
            socksPort = String(config.socksPort)
            dnsHost = config.dnsHost
            dnsPort = String(config.dnsPort)
        }
    }

    private func saveConfig() {
        let config = ConnectionConfig(
            socksHost: socksHost,
            socksPort: UInt16(socksPort) ?? 9050,
            dnsHost: dnsHost,
            dnsPort: UInt16(dnsPort) ?? 9093
        )
        do {
            try config.validate()
            try KeychainManager.shared.saveConfig(config)
        } catch {
            saveErrorMessage = error.localizedDescription
            showSaveError = true
        }
    }
}

#Preview {
    NavigationStack {
        SettingsView()
    }
}
