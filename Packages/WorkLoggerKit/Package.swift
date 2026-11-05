// swift-tools-version:6.2
import PackageDescription

// WorkLogger's logic as separate modules, each testable on its own (`swift test` here, no app needed):
// - WorkLoggerCore: models, settings, time, the worklog engine and the parsers. Foundation only.
// - WorkLoggerStore: the SQLite database and draft rebuilding.
// - WorkLoggerTestSupport: shared test fixtures for these tests; the app's tests use the same file through a
//   symlink (Tests/WorkLoggerTests/Fixture.swift), so no second copy of Core gets linked into the test bundle.
let package = Package(
    name: "WorkLoggerKit",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "WorkLoggerCore", targets: ["WorkLoggerCore"]),
        .library(name: "WorkLoggerStore", targets: ["WorkLoggerStore"]),
    ],
    targets: [
        .target(name: "WorkLoggerCore"),
        .target(name: "WorkLoggerStore", dependencies: ["WorkLoggerCore"],
                linkerSettings: [.linkedLibrary("sqlite3")]),
        .target(name: "WorkLoggerTestSupport", dependencies: ["WorkLoggerCore"]),
        .testTarget(name: "WorkLoggerCoreTests", dependencies: ["WorkLoggerCore", "WorkLoggerTestSupport"]),
        .testTarget(name: "WorkLoggerStoreTests", dependencies: ["WorkLoggerStore", "WorkLoggerTestSupport"]),
    ],
    swiftLanguageModes: [.v5]
)
