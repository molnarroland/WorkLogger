import Foundation
import WorkLoggerCore

public enum ShellError: LocalizedError {
    case failed(String, Int32, String)
    public var errorDescription: String? {
        switch self {
        case .failed(let cmd, let code, let err):
            return "\(cmd) exited with \(code): \(err.trimmingCharacters(in: .whitespacesAndNewlines))"
        }
    }
}

public enum Shell {
    /// stderr bytes handed from the reader thread back to the caller.
    private final class Box: @unchecked Sendable {
        private let lock = NSLock()
        private var data = Data()
        func set(_ d: Data) { lock.withLock { data = d } }
        func get() -> Data { lock.withLock { data } }
    }

    /// Runs a command via /usr/bin/env so `git` is resolved from PATH.
    public static func run(_ args: [String], cwd: String? = nil) throws -> String {
        let p = Process()
        p.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        p.arguments = args
        if let cwd { p.currentDirectoryURL = URL(fileURLWithPath: cwd) }
        var env = ProcessInfo.processInfo.environment
        env["PATH"] = (env["PATH"] ?? "") + ":/usr/bin:/usr/local/bin:/opt/homebrew/bin"
        env["GIT_TERMINAL_PROMPT"] = "0"
        p.environment = env
        let out = Pipe(), err = Pipe()
        p.standardOutput = out
        p.standardError = err
        try p.run()

        // Drain stderr concurrently so a chatty command can't deadlock on a full pipe.
        let errData = Box()
        let group = DispatchGroup()
        group.enter()
        DispatchQueue.global().async {
            errData.set(err.fileHandleForReading.readDataToEndOfFile())
            group.leave()
        }
        let data = out.fileHandleForReading.readDataToEndOfFile()
        group.wait()
        p.waitUntilExit()

        guard p.terminationStatus == 0 else {
            throw ShellError.failed(args.prefix(3).joined(separator: " "), p.terminationStatus,
                                    String(decoding: errData.get(), as: UTF8.self))
        }
        return String(decoding: data, as: UTF8.self)
    }
}
