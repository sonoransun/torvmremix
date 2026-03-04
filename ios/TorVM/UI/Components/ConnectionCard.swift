import SwiftUI

struct ConnectionCard: View {
    let config: ConnectionConfig

    var body: some View {
        GroupBox(label: Label("Connection", systemImage: "network")) {
            VStack(alignment: .leading, spacing: 8) {
                HStack {
                    Text("SOCKS5")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Spacer()
                    Text("\(config.socksHost):\(config.socksPort)")
                        .font(.system(.body, design: .monospaced))
                }
                HStack {
                    Text("DNS")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Spacer()
                    Text("\(config.dnsHost):\(config.dnsPort)")
                        .font(.system(.body, design: .monospaced))
                }
            }
            .padding(.top, 4)
        }
    }
}

#Preview {
    ConnectionCard(config: .direct)
        .padding()
}
