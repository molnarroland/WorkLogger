import Foundation
import WorkLoggerCore

public enum GitLogReader {
    private static let recordSep: Character = "\u{1e}"
    private static let fieldSep: Character = "\u{1f}"
    private static let headerEnd: Character = "\u{1d}"

    private static let iso: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime]
        return f
    }()

    /// Reads the user's non-merge commits on all branches between `since` and `until`.
    /// `%S` gives the ref the commit was reached from, which is used to find Jira keys in branch names.
    public static func log(repoPath: String, since: Date, until: Date, authors: [String]) throws -> [GitCommit] {
        var args = ["git", "-C", repoPath, "log", "--all", "--no-merges", "--source", "--numstat",
                    "--since=\(iso.string(from: since))", "--until=\(iso.string(from: until))",
                    "--pretty=format:%x1e%H%x1f%aI%x1f%an%x1f%ae%x1f%s%x1f%b%x1f%S%x1d"]
        args += authors.map { "--author=\($0)" }
        let output = try Shell.run(args)
        return parse(output).filter { $0.date >= since && $0.date < until }
    }

    public static func parse(_ output: String) -> [GitCommit] {
        var commits: [GitCommit] = []
        var seen = Set<String>()
        for record in output.split(separator: recordSep, omittingEmptySubsequences: true) {
            guard let sep = record.firstIndex(of: headerEnd) else { continue }
            let fields = record[..<sep].split(separator: fieldSep, omittingEmptySubsequences: false).map(String.init)
            guard fields.count >= 7, let date = iso.date(from: fields[1]), !seen.contains(fields[0]) else { continue }
            seen.insert(fields[0])

            var files: [FileStat] = []
            for line in record[record.index(after: sep)...].split(whereSeparator: \.isNewline) {
                let parts = line.split(separator: "\t", maxSplits: 2, omittingEmptySubsequences: false)
                guard parts.count == 3 else { continue }
                // Binary files show "-" instead of line counts.
                files.append(FileStat(added: Int(parts[0]) ?? 0, deleted: Int(parts[1]) ?? 0, path: String(parts[2])))
            }

            commits.append(GitCommit(hash: fields[0], date: date, author: fields[2], email: fields[3],
                                     subject: fields[4], body: fields[5].trimmingCharacters(in: .whitespacesAndNewlines),
                                     ref: fields[6], files: files))
        }
        return commits
    }
}
