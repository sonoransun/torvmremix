import Foundation

/// A log entry shared between the app and the Network Extension via App Group.
struct LogEntry: Codable, Identifiable {
    let id: UUID
    let timestamp: Date
    let level: String
    let message: String
    let source: String
}

/// Shared logger that writes to a file in the App Group container.
///
/// The Network Extension writes log entries; the app reads them.
/// Entries are stored as JSON lines for simple append-only writes.
final class TunnelLogger {

    static let shared = TunnelLogger()

    private let fileURL: URL? = {
        FileManager.default
            .containerURL(forSecurityApplicationGroupIdentifier: "group.com.torvm.shared")?
            .appendingPathComponent("tunnel.log")
    }()

    private let maxEntries = 500
    private let queue = DispatchQueue(label: "com.torvm.logger")

    private init() {}

    func log(level: String, message: String, source: String) {
        queue.async { [weak self] in
            guard let url = self?.fileURL else { return }
            let entry = LogEntry(
                id: UUID(), timestamp: Date(), level: level,
                message: message, source: source
            )
            guard let data = try? JSONEncoder().encode(entry),
                  let line = String(data: data, encoding: .utf8) else { return }

            if !FileManager.default.fileExists(atPath: url.path) {
                FileManager.default.createFile(atPath: url.path, contents: nil)
            }
            guard let handle = try? FileHandle(forWritingTo: url) else { return }
            handle.seekToEndOfFile()
            handle.write((line + "\n").data(using: .utf8)!)
            handle.closeFile()
        }
    }

    func readAll() -> [LogEntry] {
        guard let url = fileURL,
              let content = try? String(contentsOf: url, encoding: .utf8) else { return [] }
        let entries = content.split(separator: "\n").compactMap { line in
            try? JSONDecoder().decode(LogEntry.self, from: Data(line.utf8))
        }
        return Array(entries.suffix(maxEntries).reversed())
    }

    func clear() {
        guard let url = fileURL else { return }
        try? "".write(to: url, atomically: true, encoding: .utf8)
    }
}
