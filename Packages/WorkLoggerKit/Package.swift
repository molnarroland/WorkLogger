// swift-tools-version:6.2
import PackageDescription

// WorkLogger's logic as separate modules, each testable on its own (`swift test` here, no app needed):
// - WorkLoggerCore: models, settings, time, the worklog engine and the parsers. Foundation only.
// - WorkLoggerStore: the SQLite database and draft rebuilding.
// - WorkLoggerGit, WorkLoggerCalendar, WorkLoggerTempo: the outside world (git CLI, EventKit and Google, Jira/Tempo
//   REST), each behind a protocol so the app and its tests can swap in fakes.
// - WorkLoggerWeb: reading Google Chat and HRmaster from a signed-in page (`PageDriver`; the app's WebKit session
//   is the real one, tests use a fake page).
// - WorkLoggerTestSupport: shared test fixtures for these tests; the app's tests use the same file through a
//   symlink (Tests/WorkLoggerTests/Fixture.swift), so no second copy of Core gets linked into the test bundle.
let package = Package(
    name: "WorkLoggerKit",
    defaultLocalization: "en",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "WorkLoggerCore", targets: ["WorkLoggerCore"]),
        .library(name: "WorkLoggerStore", targets: ["WorkLoggerStore"]),
        .library(name: "WorkLoggerGit", targets: ["WorkLoggerGit"]),
        .library(name: "WorkLoggerCalendar", targets: ["WorkLoggerCalendar"]),
        .library(name: "WorkLoggerTempo", targets: ["WorkLoggerTempo"]),
        .library(name: "WorkLoggerWeb", targets: ["WorkLoggerWeb"]),
    ],
    targets: [
        .target(name: "WorkLoggerCore", resources: [.process("Localizable.xcstrings")]),
        .target(name: "WorkLoggerStore", dependencies: ["WorkLoggerCore"],
                linkerSettings: [.linkedLibrary("sqlite3")]),
        .target(name: "WorkLoggerGit", dependencies: ["WorkLoggerCore"]),
        .target(name: "WorkLoggerCalendar", dependencies: ["WorkLoggerCore"],
                linkerSettings: [.linkedFramework("EventKit")]),
        .target(name: "WorkLoggerTempo", dependencies: ["WorkLoggerCore", "WorkLoggerStore"], resources: [.process("Localizable.xcstrings")]),
        .target(name: "WorkLoggerWeb", dependencies: ["WorkLoggerCore"], resources: [.process("Localizable.xcstrings")]),
        .target(name: "WorkLoggerTestSupport", dependencies: ["WorkLoggerCore", "WorkLoggerTempo"]),
        .testTarget(name: "WorkLoggerCoreTests", dependencies: ["WorkLoggerCore", "WorkLoggerTestSupport"]),
        .testTarget(name: "WorkLoggerStoreTests", dependencies: ["WorkLoggerStore", "WorkLoggerTestSupport"]),
        .testTarget(name: "WorkLoggerGitTests", dependencies: ["WorkLoggerGit", "WorkLoggerTestSupport"]),
        .testTarget(name: "WorkLoggerCalendarTests", dependencies: ["WorkLoggerCalendar", "WorkLoggerTestSupport"]),
        .testTarget(name: "WorkLoggerTempoTests", dependencies: ["WorkLoggerTempo", "WorkLoggerTestSupport"]),
        .testTarget(name: "WorkLoggerWebTests", dependencies: ["WorkLoggerWeb", "WorkLoggerTestSupport"]),
    ],
    swiftLanguageModes: [.v6]
)
