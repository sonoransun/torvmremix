import SwiftUI

@main
struct TorVMApp: App {
    @Environment(\.scenePhase) private var scenePhase

    init() {
        if !KeychainManager.shared.configExists() {
            try? KeychainManager.shared.saveConfig(.direct)
        }
        BiometricAuthManager.shared.loadPreferences()
    }

    var body: some Scene {
        WindowGroup {
            HomeView()
        }
        .onChange(of: scenePhase) { _, newPhase in
            if newPhase == .background {
                BiometricAuthManager.shared.clearCache()
            }
        }
    }
}
