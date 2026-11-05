import os

/// Unified logging, one category per area. In Console.app filter on subsystem `dev.worklogger`, e.g. category
/// `web` to follow a Google Chat or HRmaster fetch, or `store` for database migrations.
public enum Log {
    private static let subsystem = "dev.worklogger"
    public static let app = Logger(subsystem: subsystem, category: "app")
    public static let store = Logger(subsystem: subsystem, category: "store")
    public static let git = Logger(subsystem: subsystem, category: "git")
    public static let calendar = Logger(subsystem: subsystem, category: "calendar")
    public static let tempo = Logger(subsystem: subsystem, category: "tempo")
    public static let web = Logger(subsystem: subsystem, category: "web")
}
