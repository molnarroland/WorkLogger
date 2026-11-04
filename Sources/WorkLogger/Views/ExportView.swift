import SwiftUI

/// Export popover behind the toolbar's share icon.
struct ExportView: View {
    @Environment(AppState.self) private var state
    @State private var includeExported = false
    @State private var onlyReviewed = false
    @State private var confirmPush = false

    private var selection: [Worklog] {
        state.worklogs.filter { w in
            (includeExported || w.status != .exported) && (!onlyReviewed || w.status == .reviewed)
        }
    }

    var body: some View {
        let logs = selection
        let ready = TempoExport.exportable(logs)
        let missing = logs.filter { $0.issueKey == nil && $0.duration > 0 }
        let total = ready.reduce(0) { $0 + $1.duration }

        VStack(alignment: .leading, spacing: 14) {
            VStack(alignment: .leading, spacing: 2) {
                Text("Export \(state.fromDay) – \(state.toDay)").font(.headline)
                Text("\(ready.count) entries · \(DurationFormat.string(total))").foregroundStyle(.secondary)
                if !missing.isEmpty {
                    Label("\(missing.count) entries without issue are skipped", systemImage: "exclamationmark.triangle")
                        .foregroundStyle(.orange).font(.callout)
                }
            }
            VStack(alignment: .leading, spacing: 4) {
                Toggle("Only entries marked reviewed", isOn: $onlyReviewed)
                Toggle("Include entries already exported", isOn: $includeExported)
            }

            Divider()
            GlassEffectContainer(spacing: 8) {
                VStack(alignment: .leading, spacing: 10) {
                    Text("Claude in Chrome").font(.subheadline.weight(.semibold))
                    Text("A Markdown checklist with instructions: give it to Claude and it fills your Tempo timesheet in the browser.")
                        .font(.callout).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
                    HStack {
                        Button { state.copyMarkdown(ready) } label: { Label("Copy Markdown", systemImage: "doc.on.doc") }
                            .buttonStyle(.glassProminent)
                        Button { state.exportMarkdown(ready) } label: { Label("Save .md…", systemImage: "doc.text") }
                            .buttonStyle(.glass)
                    }
                    .disabled(ready.isEmpty)

                    Text("Tempo API").font(.subheadline.weight(.semibold)).padding(.top, 6)
                    HStack {
                        Button { confirmPush = true } label: { Label("Push to Tempo…", systemImage: "paperplane") }
                            .buttonStyle(.glass)
                            .disabled(ready.isEmpty || state.isBusy)
                        Button("Mark as exported") { state.markExported(ready) }
                            .buttonStyle(.glass)
                            .disabled(ready.isEmpty)
                    }

                    Text("Files").font(.subheadline.weight(.semibold)).padding(.top, 6)
                    HStack {
                        Button("CSV…") { state.exportCSV(ready) }.buttonStyle(.glass)
                        Button("JSON…") { state.exportJSON(ready) }.buttonStyle(.glass)
                    }
                    .disabled(ready.isEmpty)
                }
            }
            Text("After entering the worklogs in Tempo yourself (or via Claude), mark them exported so they aren't sent twice.")
                .font(.caption).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
        }
        .padding(18)
        .frame(width: 380)
        .confirmationDialog("Send \(ready.count) worklogs (\(DurationFormat.string(total))) to Tempo?",
                            isPresented: $confirmPush) {
            Button("Send to Tempo") { Task { await state.pushToTempo(ready) } }
            Button("Cancel", role: .cancel) {}
        } message: {
            Text("They are created under your Jira account for \(state.fromDay) – \(state.toDay).")
        }
    }
}

#Preview {
    ExportView()
        .environment(AppState.preview)
}
