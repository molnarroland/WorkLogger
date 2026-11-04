// swift-tools-version:6.2
import PackageDescription
import Foundation

// Embeds Info.plist into the binary so macOS can show the calendar permission prompt even with `swift run`.
let infoPlist = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
    .appendingPathComponent("Support/Info.plist").path

let package = Package(
    name: "WorkLogger",
    platforms: [.macOS(.v26)],
    targets: [
        .executableTarget(
            name: "WorkLogger",
            path: "Sources/WorkLogger",
            exclude: ["AppIcon.icon"],
            linkerSettings: [
                .linkedLibrary("sqlite3"),
                .linkedFramework("EventKit"),
                .unsafeFlags(["-Xlinker", "-sectcreate", "-Xlinker", "__TEXT",
                              "-Xlinker", "__info_plist", "-Xlinker", infoPlist])
            ]
        ),
        .testTarget(
            name: "WorkLoggerTests",
            dependencies: ["WorkLogger"],
            path: "Tests/WorkLoggerTests"
        )
    ],
    swiftLanguageModes: [.v5]
)
