import Foundation
import WorkLoggerCore
import WorkLoggerStore

enum ShellError: LocalizedError {
    case failed(String, Int32, String)
    var errorDescription: String? {
        switch self {
        case .failed(let cmd, let code, let err):
            return "\(cmd) exited with \(code): \(err.trimmingCharacters(in: .whitespacesAndNewlines))"
        }
    }
}

enum Shell {
    /// Runs a command via /usr/bin/env so `git` is resolved from PATH.
    static func run(_ args: [String], cwd: String? = nil) throws -> String {
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
        var errData = Data()
        let group = DispatchGroup()
        group.enter()
        DispatchQueue.global().async {
            errData = err.fileHandleForReading.readDataToEndOfFile()
            group.leave()
        }
        let data = out.fileHandleForReading.readDataToEndOfFile()
        group.wait()
        p.waitUntilExit()

        guard p.terminationStatus == 0 else {
            throw ShellError.failed(args.prefix(3).joined(separator: " "), p.terminationStatus,
                                    String(decoding: errData, as: UTF8.self))
        }
        return String(decoding: data, as: UTF8.self)
    }
}
