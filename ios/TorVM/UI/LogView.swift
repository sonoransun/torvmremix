import SwiftUI
import UIKit

struct LogView: View {
    @State private var entries: [LogEntry] = []
    @State private var searchText = ""
    @State private var selectedLevel = "all"
    @State private var showShareSheet = false

    private let timer = Timer.publish(every: 2, on: .main, in: .common).autoconnect()

    private let logLevels = ["all", "error", "info", "debug"]

    private var filteredEntries: [LogEntry] {
        entries.filter { entry in
            let matchesLevel = selectedLevel == "all" || entry.level == selectedLevel
            let matchesSearch = searchText.isEmpty
                || entry.message.localizedCaseInsensitiveContains(searchText)
                || entry.source.localizedCaseInsensitiveContains(searchText)
            return matchesLevel && matchesSearch
        }
    }

    private var shareContent: String {
        let formatter = DateFormatter()
        formatter.dateFormat = "HH:mm:ss"
        return filteredEntries.reversed().map { entry in
            "[\(formatter.string(from: entry.timestamp))] [\(entry.source)] [\(entry.level)] \(entry.message)"
        }.joined(separator: "\n")
    }

    var body: some View {
        ScrollViewReader { proxy in
            List {
                ForEach(filteredEntries) { entry in
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
                    .id(entry.id)
                    .accessibilityLabel("\(entry.source) \(entry.level) log at \(entry.timestamp.formatted(date: .omitted, time: .shortened)): \(entry.message)")
                }
            }
            .listStyle(.plain)
            .navigationTitle("Logs")
            .searchable(text: $searchText, prompt: "Search logs")
            .toolbar {
                ToolbarItemGroup(placement: .navigationBarTrailing) {
                    Menu {
                        Picker("Log Level", selection: $selectedLevel) {
                            ForEach(logLevels, id: \.self) { level in
                                Text(level.capitalized).tag(level)
                            }
                        }
                    } label: {
                        Image(systemName: "line.3.horizontal.decrease.circle")
                    }
                    .accessibilityLabel("Filter by log level")
                    .accessibilityHint("Currently showing \(selectedLevel) logs. Double tap to change filter.")

                    Button {
                        showShareSheet = true
                    } label: {
                        Image(systemName: "square.and.arrow.up")
                    }
                    .accessibilityLabel("Share logs")
                    .accessibilityHint("Double tap to share the current log entries.")

                    Button {
                        if let lastEntry = filteredEntries.first {
                            withAnimation {
                                proxy.scrollTo(lastEntry.id, anchor: .top)
                            }
                        }
                    } label: {
                        Image(systemName: "arrow.down.to.line")
                    }
                    .accessibilityLabel("Jump to latest")
                    .accessibilityHint("Double tap to scroll to the most recent log entry.")

                    Button("Clear") {
                        TunnelLogger.shared.clear()
                        entries = []
                    }
                    .accessibilityLabel("Clear logs")
                    .accessibilityHint("Double tap to delete all log entries.")
                }
            }
            .onAppear { entries = TunnelLogger.shared.readAll() }
            .onReceive(timer) { _ in entries = TunnelLogger.shared.readAll() }
            .sheet(isPresented: $showShareSheet) {
                ShareSheet(activityItems: [shareContent])
            }
        }
    }

    private func sourceBadgeColor(_ source: String) -> Color {
        switch source {
        case "tunnel": return .blue
        case "app": return .green
        default: return .gray
        }
    }
}

private struct ShareSheet: UIViewControllerRepresentable {
    let activityItems: [Any]

    func makeUIViewController(context: Context) -> UIActivityViewController {
        UIActivityViewController(activityItems: activityItems, applicationActivities: nil)
    }

    func updateUIViewController(_ uiViewController: UIActivityViewController, context: Context) {}
}

#Preview {
    NavigationStack {
        LogView()
    }
}
