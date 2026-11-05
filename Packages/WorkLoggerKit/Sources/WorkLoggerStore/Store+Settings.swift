import Foundation
import WorkLoggerCore

/// App settings, stored as one JSON value.
public extension Database {

    // MARK: Settings

    func loadSettings() -> AppSettings {
        guard let raw = (try? query("SELECT value FROM settings WHERE key = 'app'"))?.first?.string("value"),
              let data = raw.data(using: .utf8),
              let settings = AppSettings.decodeMerging(data)
        else { return AppSettings() }
        return settings
    }

    func saveSettings(_ s: AppSettings) throws {
        let json = String(decoding: try JSONEncoder().encode(s), as: UTF8.self)
        try run("INSERT INTO settings(key, value) VALUES('app', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
                [.text(json)])
    }
}
