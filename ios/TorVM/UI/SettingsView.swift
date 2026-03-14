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

    private var cacheDurationText: String {
        switch bioAuth.cacheDuration {
        case 60: return "1 minute"
        case 300: return "5 minutes"
        case 900: return "15 minutes"
        case 0: return "Never"
        default: return "\(Int(bioAuth.cacheDuration)) seconds"
        }
    }

    var body: some View {
        Form {
            Section("Presets") {
                HStack(spacing: 12) {
                    Button("Direct (TAP)") {
                        applyPreset(.direct)
                    }
                    .buttonStyle(.bordered)
                    .accessibilityLabel("Direct TAP preset")
                    .accessibilityHint("Double tap to apply the direct TAP connection preset.")

                    Button("WiFi/LAN") {
                        applyPreset(.wifiDefault)
                    }
                    .buttonStyle(.bordered)
                    .accessibilityLabel("WiFi LAN preset")
                    .accessibilityHint("Double tap to apply the WiFi and LAN connection preset.")
                }
            }

            Section("Connection") {
                TextField("SOCKS5 Host", text: $socksHost)
                    .textContentType(.URL)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                    .accessibilityLabel("SOCKS5 Host")
                    .accessibilityValue(socksHost.isEmpty ? "empty" : socksHost)
                TextField("SOCKS5 Port", text: $socksPort)
                    .keyboardType(.numberPad)
                    .accessibilityLabel("SOCKS5 Port")
                    .accessibilityValue(socksPort.isEmpty ? "empty" : socksPort)
                TextField("DNS Host", text: $dnsHost)
                    .textContentType(.URL)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                    .accessibilityLabel("DNS Host")
                    .accessibilityValue(dnsHost.isEmpty ? "empty" : dnsHost)
                TextField("DNS Port", text: $dnsPort)
                    .keyboardType(.numberPad)
                    .accessibilityLabel("DNS Port")
                    .accessibilityValue(dnsPort.isEmpty ? "empty" : dnsPort)
            }

            Section("Security") {
                Toggle("Require biometric for VPN", isOn: Binding(
                    get: { bioAuth.requireBiometricForVPN },
                    set: {
                        bioAuth.requireBiometricForVPN = $0
                        bioAuth.savePreferences()
                    }
                ))
                .accessibilityLabel("Require biometric for VPN")
                .accessibilityValue(bioAuth.requireBiometricForVPN ? "enabled" : "disabled")
                .accessibilityHint("Double tap to \(bioAuth.requireBiometricForVPN ? "disable" : "enable") biometric requirement for VPN toggling.")

                Toggle("Require biometric for settings", isOn: Binding(
                    get: { bioAuth.requireBiometricForSettings },
                    set: {
                        bioAuth.requireBiometricForSettings = $0
                        bioAuth.savePreferences()
                    }
                ))
                .accessibilityLabel("Require biometric for settings")
                .accessibilityValue(bioAuth.requireBiometricForSettings ? "enabled" : "disabled")
                .accessibilityHint("Double tap to \(bioAuth.requireBiometricForSettings ? "disable" : "enable") biometric requirement for saving settings.")

                Picker("Cache duration", selection: cacheDurationBinding) {
                    Text("1 minute").tag(TimeInterval(60))
                    Text("5 minutes").tag(TimeInterval(300))
                    Text("15 minutes").tag(TimeInterval(900))
                    Text("Never").tag(TimeInterval(0))
                }
                .accessibilityLabel("Biometric cache duration")
                .accessibilityValue(cacheDurationText)
                .accessibilityHint("Currently set to \(cacheDurationText). Double tap to change.")

                HStack {
                    Text("Biometric type")
                    Spacer()
                    Text(biometricLabel)
                        .foregroundStyle(.secondary)
                }
                .accessibilityElement(children: .combine)
                .accessibilityLabel("Biometric type: \(biometricLabel)")
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
                .accessibilityLabel("Save settings")
                .accessibilityHint("Double tap to save the current connection and security settings.")
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
