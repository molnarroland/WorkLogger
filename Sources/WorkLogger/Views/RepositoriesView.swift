import SwiftUI
import WorkLoggerCore
import WorkLoggerStore

struct RepositoriesView: View {
    @Environment(AppState.self) private var state
    @State private var newProjectKey = ""
    @State private var newProjectIssue = ""

    var body: some View {
        @Bindable var state = state
        VStack(alignment: .leading, spacing: 0) {
            // One compact setup card: folder, your commit emails and skipped folders as chips.
            VStack(alignment: .leading, spacing: 10) {
                SetupRow("Workspace") {
                    Text(state.settings.workspaceRoot.isEmpty ? "No folder chosen" : state.settings.workspaceRoot)
                        .font(.body.monospaced())
                        .foregroundStyle(state.settings.workspaceRoot.isEmpty ? Color.secondary : Color.primary)
                        .lineLimit(1).truncationMode(.middle)
                    Spacer()
                    Button("Choose folder…") { state.chooseWorkspace() }.buttonStyle(.glass)
                    Button("Scan again") { state.scanWorkspace() }.buttonStyle(.glassProminent)
                        .disabled(state.settings.workspaceRoot.isEmpty)
                }
                SetupRow("Your commit emails") {
                    ChipList(values: $state.settings.authorEmails, placeholder: "you@company.com", highlighted: true)
                }
                SetupRow("Skipped folders") {
                    ChipList(values: $state.settings.excludedFolders, placeholder: "name or ~/path")
                        .help("Never scanned. A name like \"archive\" matches any folder with that name; a path like ~/Work/old matches that folder. Repositories already listed inside them are removed.")
                }
                DisclosureGroup("Jira projects (\(state.projects.count))") {
                    VStack(alignment: .leading, spacing: 6) {
                        ForEach(state.projects) { p in
                            HStack {
                                Text(p.key).font(.body.monospaced().weight(.medium)).frame(width: 90, alignment: .leading)
                                Text(p.defaultIssue ?? "No catch-all issue")
                                    .foregroundStyle(p.defaultIssue == nil ? Color.orange : Color.primary)
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
                        .textFieldStyle(.roundedBorder)
                        Text("Tempo logs time to issues, so a repository assigned only to a project uses that project's catch-all issue.")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                    .padding(.top, 6)
                }
            }
            .padding(14)
            .glassEffect(.regular, in: .rect(cornerRadius: 18))
            .padding(16)

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

/// A labelled line of the setup card.
private struct SetupRow<Content: View>: View {
    let title: String
    @ViewBuilder var content: Content
    init(_ title: String, @ViewBuilder content: () -> Content) { self.title = title; self.content = content() }

    var body: some View {
        HStack(spacing: 8) {
            Text(title).foregroundStyle(.secondary).frame(width: 150, alignment: .leading)
            content
        }
    }
}

/// Values as removable chips, with a field to add one (Return adds; commas split several).
struct ChipList: View {
    @Binding var values: [String]
    var placeholder = "Add"
    var highlighted = false
    @State private var text = ""

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 6) {
                ForEach(values, id: \.self) { v in
                    HStack(spacing: 4) {
                        Text(v).lineLimit(1)
                        Button { values.removeAll { $0 == v } } label: { Image(systemName: "xmark").font(.caption2.weight(.bold)) }
                            .buttonStyle(.plain).foregroundStyle(.secondary)
                            .help("Remove \(v)")
                    }
                    .font(.callout)
                    .padding(.horizontal, 10).padding(.vertical, 4)
                    .glassEffect(highlighted ? .regular.tint(.accentColor.opacity(0.18)) : .regular, in: .capsule)
                }
                TextField(placeholder, text: $text)
                    .textFieldStyle(.roundedBorder).frame(width: 170)
                    .onSubmit(add)
            }
            .padding(.vertical, 2)
        }
    }

    private func add() {
        let new = text.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty && !values.contains($0) }
        values += new
        text = ""
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
            // What each repository brings this month; a switch can stay off for the quiet ones.
            Text(repo.enabled ? "\(state.commitCounts[repo.id] ?? 0) commits" : "—")
                .monospacedDigit().foregroundStyle(.secondary).frame(width: 90, alignment: .trailing)
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
