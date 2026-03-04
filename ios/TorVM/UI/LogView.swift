import SwiftUI

struct LogView: View {
    @State private var entries: [LogEntry] = []

    private let timer = Timer.publish(every: 2, on: .main, in: .common).autoconnect()

    var body: some View {
        List(entries) { entry in
            HStack(alignment: .top, spacing: 8) {
                Text(entry.timestamp, style: .time)
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .frame(width: 60, alignment: .leading)

                Text(entry.source)
                    .font(.caption2)
                    .fontWeight(.medium)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(sourceBadgeColor(entry.source))
                    .foregroundStyle(.white)
                    .clipShape(Capsule())

                Text(entry.message)
                    .font(.system(.caption, design: .monospaced))
                    .foregroundStyle(entry.level == "error" ? .red : .primary)
            }
        }
        .listStyle(.plain)
        .navigationTitle("Logs")
        .toolbar {
            ToolbarItem(placement: .navigationBarTrailing) {
                Button("Clear") {
                    TunnelLogger.shared.clear()
                    entries = []
                }
            }
        }
        .onAppear { entries = TunnelLogger.shared.readAll() }
        .onReceive(timer) { _ in entries = TunnelLogger.shared.readAll() }
    }

    private func sourceBadgeColor(_ source: String) -> Color {
        switch source {
        case "tunnel": return .blue
        case "app": return .green
        default: return .gray
        }
    }
}

#Preview {
    NavigationStack {
        LogView()
    }
}
