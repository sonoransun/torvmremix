import SwiftUI

struct FirewallInfoView: View {
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                Label("Per-App VPN Not Available", systemImage: "exclamationmark.shield")
                    .font(.title3)
                    .fontWeight(.semibold)

                Text("iOS does not allow VPN apps to selectively route traffic on a per-app basis. When the TorVM tunnel is active, all network traffic from every app on the device is routed through the Tor network.")

                Text("This is a platform limitation enforced by Apple. Unlike Android, where TorVM's AppFirewall feature lets you choose which apps use the VPN and which bypass it, iOS treats the VPN tunnel as an all-or-nothing connection.")

                GroupBox(label: Label("Android Comparison", systemImage: "arrow.left.arrow.right")) {
                    Text("The Android companion app provides an AppFirewall screen where individual apps can be included or excluded from the VPN tunnel using Android's per-app VPN API (VpnService.Builder.addAllowedApplication / addDisallowedApplication). This granularity is not available on iOS.")
                        .font(.callout)
                }

                GroupBox(label: Label("What This Means", systemImage: "info.circle")) {
                    VStack(alignment: .leading, spacing: 8) {
                        BulletPoint("All apps route through Tor when connected")
                        BulletPoint("No way to exclude specific apps from the tunnel")
                        BulletPoint("Disconnect the VPN to use direct connections")
                    }
                    .font(.callout)
                }
            }
            .padding()
        }
        .navigationTitle("Firewall")
    }
}

private struct BulletPoint: View {
    let text: String
    init(_ text: String) { self.text = text }

    var body: some View {
        HStack(alignment: .top, spacing: 6) {
            Text("\u{2022}")
            Text(text)
        }
    }
}

#Preview {
    NavigationStack {
        FirewallInfoView()
    }
}
