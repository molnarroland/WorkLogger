import EventKit
import Foundation
import SwiftUI
import UniformTypeIdentifiers
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

/// Repositories and Jira projects.
extension AppState {
    // MARK: Repositories

    func chooseWorkspace() {
        guard let url = services.files.chooseFolder() else { return }
        settings.workspaceRoot = url.path
        scanWorkspace()
    }

    func scanWorkspace() {
        guard !settings.workspaceRoot.isEmpty else {
            status = String(localized: "Choose your workspace folder first.")
            return
        }
        let root = URL(fileURLWithPath: (settings.workspaceRoot as NSString).expandingTildeInPath)
        let found = services.git.findRepos(root: root, excluded: settings.excludedFolders)
        attempt("Scanning") {
            for url in found { try db.addRepo(path: url.path, name: url.lastPathComponent) }
        }
        reloadAll()
        removeHiddenRepos()
        status = String(localized: "Found \(found.count) repositories in \(root.lastPathComponent).")
    }

    /// Drops listed repositories that sit inside a hidden folder.
    func removeHiddenRepos() {
        let hidden = repos.filter { GitScanner.isExcluded($0.path, by: settings.excludedFolders) }
        guard !hidden.isEmpty else { return }
        attempt("Hiding repositories") {
            for repo in hidden { try db.deleteRepo(repo.id) }
            repos = try db.repos()
        }
    }

    func update(_ repo: Repo) {
        var r = repo
        r.issueKey = JiraKey.normalize(r.issueKey)
        r.projectKey = JiraKey.normalize(r.projectKey)
        attempt("Saving repository") { try db.updateRepo(r) }
    }

    func removeRepo(_ repo: Repo) {
        attempt("Removing repository") { try db.deleteRepo(repo.id) }
        reloadAll()
    }

    func saveProject(_ p: ProjectMapping) {
        guard let key = JiraKey.normalize(p.key) else { return }
        attempt("Saving project") {
            try db.upsertProject(ProjectMapping(key: key, defaultIssue: JiraKey.normalize(p.defaultIssue)))
        }
        projects = (try? db.projects()) ?? projects
    }

    func removeProject(_ p: ProjectMapping) {
        attempt("Removing project") { try db.deleteProject(p.key) }
        projects = (try? db.projects()) ?? projects
    }
}
