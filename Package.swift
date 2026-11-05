// swift-tools-version:6.2
import PackageDescription
import Foundation

// Embeds Info.plist into the binary so macOS can show the calendar permission prompt even with `swift run`.
let infoPlist = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
    .appendingPathComponent("Support/Info.plist").path

let package = Package(
    name: "WorkLogger",
    platforms: [.macOS(.v26)],
    dependencies: [.package(path: "Packages/WorkLoggerKit")],
    targets: [
        .executableTarget(
            name: "WorkLogger",
            dependencies: [.product(name: "WorkLoggerCore", package: "WorkLoggerKit"),
                           .product(name: "WorkLoggerStore", package: "WorkLoggerKit"),
                           .product(name: "WorkLoggerGit", package: "WorkLoggerKit"),
                           .product(name: "WorkLoggerCalendar", package: "WorkLoggerKit"),
                           .product(name: "WorkLoggerTempo", package: "WorkLoggerKit"),
                           .product(name: "WorkLoggerWeb", package: "WorkLoggerKit")],
            path: "Sources/WorkLogger",
            exclude: ["AppIcon.icon"],
            linkerSettings: [
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
    swiftLanguageModes: [.v6]
)
