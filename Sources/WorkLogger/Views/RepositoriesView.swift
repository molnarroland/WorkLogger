import SwiftUI

struct RepositoriesView: View {
    @Environment(AppState.self) private var state
    @State private var newProjectKey = ""
    @State private var newProjectIssue = ""

    var body: some View {
        @Bindable var state = state
        VStack(alignment: .leading, spacing: 0) {
            Form {
                Section("Workspace") {
                    HStack {
                        Text(state.settings.workspaceRoot.isEmpty ? "No folder chosen" : state.settings.workspaceRoot)
                            .foregroundStyle(state.settings.workspaceRoot.isEmpty ? Color.secondary : Color.primary)
                            .lineLimit(1).truncationMode(.middle)
                        Spacer()
                        Button("Choose folder…") { state.chooseWorkspace() }
                        Button("Scan again") { state.scanWorkspace() }
                            .disabled(state.settings.workspaceRoot.isEmpty)
                    }
                    ListField(title: "Your commit emails (comma separated)", values: $state.settings.authorEmails)
                    ListField(title: "Hidden folders (names or paths, comma separated)", values: $state.settings.excludedFolders)
                        .help("Never scanned. A name like \"archive\" matches any folder with that name; a path like ~/Work/old matches that folder. Repositories already listed inside them are removed.")
                }
                Section {
                    ForEach(state.projects) { p in
                        HStack {
                            Text(p.key).bold().frame(width: 90, alignment: .leading)
                            Text(p.defaultIssue ?? "No catch-all issue").foregroundStyle(p.defaultIssue == nil ? Color.orange : Color.primary)
                            Spacer()
                            Button(role: .destructive) { state.removeProject(p) } label: { Image(systemName: "trash") }
                                .buttonStyle(.borderless)
                        }
                    }
                    HStack {
                        TextField("Project key, e.g. TAO", text: $newProjectKey).frame(width: 180)
                        TextField("Catch-all issue, e.g. TAO-1", text: $newProjectIssue).frame(width: 220)
                        Button("Add project") {
                            state.saveProject(ProjectMapping(key: newProjectKey, defaultIssue: newProjectIssue))
                            newProjectKey = ""
                            newProjectIssue = ""
                        }
                        .disabled(newProjectKey.isEmpty)
                    }
                } header: {
                    Text("Jira projects")
                } footer: {
                    Text("Tempo logs time to issues, so a repository assigned only to a project uses that project's catch-all issue.")
                }
            }
            .formStyle(.grouped)
            .frame(maxHeight: 330)

            Divider()
            HStack {
                Text("Repositories").font(.headline)
                Text("Commit message key → branch key → repository issue → project catch-all issue")
                    .font(.callout).foregroundStyle(.secondary)
            }
            .padding(.horizontal, 16).padding(.top, 10)

            if state.repos.isEmpty {
                ContentUnavailableView("No repositories yet", systemImage: "folder.badge.questionmark",
                                       description: Text("Choose your workspace folder to find its git repositories."))
            } else {
                List {
                    ForEach($state.repos) { $repo in
                        RepoRow(repo: $repo)
                    }
                }
                .listStyle(.inset)
            }
        }
    }
}

struct RepoRow: View {
    @Environment(AppState.self) private var state
    @Binding var repo: Repo

    private func binding(_ kp: WritableKeyPath<Repo, String?>) -> Binding<String> {
        Binding(get: { repo[keyPath: kp] ?? "" }, set: { repo[keyPath: kp] = $0.isEmpty ? nil : $0.uppercased() })
    }

    var body: some View {
        HStack(spacing: 12) {
            Toggle("", isOn: $repo.enabled).labelsHidden().help("Include this repository when collecting")
            VStack(alignment: .leading) {
                Text(repo.name).bold()
                Text(repo.path).font(.caption).foregroundStyle(.secondary).lineLimit(1).truncationMode(.middle)
            }
            Spacer()
            TextField("Default issue", text: binding(\.issueKey)).frame(width: 130)
            TextField("Project", text: binding(\.projectKey)).frame(width: 90)
            Button(role: .destructive) { state.removeRepo(repo) } label: { Image(systemName: "trash") }
                .buttonStyle(.borderless)
                .help("Remove from the list (the repository on disk is not touched)")
        }
        .opacity(repo.enabled ? 1 : 0.55)
        .onChange(of: repo) { state.update(repo) }
    }
}

#Preview {
    RepositoriesView()
        .environment(AppState.preview)
        .frame(width: 900, height: 600)
}
