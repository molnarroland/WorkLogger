import Foundation

enum GitScanner {
    private static let skipped: Set<String> = ["node_modules", "vendor", "build", "dist", ".build",
                                               "Pods", "DerivedData", "storage", "target"]

    /// Finds git repositories (and worktrees) below a workspace folder.
    static func findRepos(root: URL, maxDepth: Int = 4) -> [URL] {
        var result: [URL] = []
        let fm = FileManager.default

        func walk(_ url: URL, depth: Int) {
            if fm.fileExists(atPath: url.appendingPathComponent(".git").path) {
                result.append(url)
                return   // don't descend into a repo (skips submodules and vendored repos)
            }
            guard depth < maxDepth,
                  let items = try? fm.contentsOfDirectory(at: url, includingPropertiesForKeys: [.isDirectoryKey],
                                                          options: [.skipsHiddenFiles, .skipsPackageDescendants])
            else { return }
            for item in items where !skipped.contains(item.lastPathComponent) {
                if (try? item.resourceValues(forKeys: [.isDirectoryKey]).isDirectory) == true {
                    walk(item, depth: depth + 1)
                }
            }
        }

        walk(root, depth: 0)
        return result.sorted { $0.lastPathComponent.localizedCaseInsensitiveCompare($1.lastPathComponent) == .orderedAscending }
    }

    static func globalUserEmail() -> String? {
        let email = (try? Shell.run(["git", "config", "--global", "user.email"]))?
            .trimmingCharacters(in: .whitespacesAndNewlines)
        return (email?.isEmpty ?? true) ? nil : email
    }
}
