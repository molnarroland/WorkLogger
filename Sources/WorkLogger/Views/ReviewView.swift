import SwiftUI

struct ReviewView: View {
    @Environment(AppState.self) private var state

    var body: some View {
        @Bindable var state = state
        VStack(spacing: 0) {
            PaneHeader(title: "Review", subtitle: "\(state.worklogs.count) entries") {
                Button("Save changes") { state.saveDirty() }
                    .buttonStyle(.glass)
                    .disabled(state.dirty.isEmpty)
            }

            if state.worklogs.isEmpty {
                ContentUnavailableView {
                    Label("No worklogs in this range", systemImage: "clock")
                } description: {
                    Text("Pick a range and choose Collect to read your commits and meetings.")
                }
            } else {
                List {
                    ForEach(state.days, id: \.self) { day in
                        Section {
                            ForEach($state.worklogs) { $log in
                                if log.day == day { WorklogRow(log: $log) }
                            }
                        } header: {
                            DayHeader(day: day)
                        }
                    }
                }
                .listStyle(.inset)
            }
        }
    }
}

struct DayHeader: View {
    @Environment(AppState.self) private var state
    let day: String

    var body: some View {
        let total = state.total(day: day)
        let target = state.target(day: day)
        let missing = state.worklogs.filter { $0.day == day && $0.issueKey == nil && $0.duration > 0 }.count
        HStack(spacing: 12) {
            Text(DayKey.title(day)).font(.headline)
            if let record = state.dayRecords[day], !record.label.isEmpty {
                Text(record.label).foregroundStyle(record.status == .workday ? Color.secondary : Color.blue)
            }
            Text(DurationFormat.string(total)).monospacedDigit()
            if let target {
                Text("of \(DurationFormat.string(target))")
                    .foregroundStyle(total == target ? Color.secondary : Color.orange)
            }
            if missing > 0 {
                Label("\(missing) without issue", systemImage: "exclamationmark.triangle")
                    .foregroundStyle(.orange)
            }
            Spacer()
            Button("Mark reviewed") { state.setStatus(.reviewed, forAllIn: day) }
                .buttonStyle(.borderless)
            Button {
                state.addManual(day: day)
            } label: {
                Label("Add entry", systemImage: "plus")
            }
            .buttonStyle(.borderless)
        }
        .padding(.vertical, 4)
    }
}

struct WorklogRow: View {
    @Environment(AppState.self) private var state
    @Binding var log: Worklog
    @State private var startText = ""
    @State private var durationText = ""
    @FocusState private var focus: Field?

    enum Field { case start, duration }

    private var issueBinding: Binding<String> {
        Binding(get: { log.issueKey ?? "" }, set: { log.issueKey = JiraKey.normalize($0) })
    }

    var body: some View {
        HStack(spacing: 10) {
            Image(systemName: log.source.icon)
                .foregroundStyle(.secondary)
                .frame(width: 18)
                .help(log.source.label)
            TextField("09:00", text: $startText)
                .frame(width: 54)
                .monospacedDigit()
                .focused($focus, equals: .start)
                .onSubmit(applyStart)
            TextField("1h 30m", text: $durationText)
                .frame(width: 72)
                .monospacedDigit()
                .focused($focus, equals: .duration)
                .onSubmit(applyDuration)
            TextField("ISSUE-1", text: issueBinding)
                .frame(width: 110)
                .foregroundStyle(log.issueKey == nil ? Color.orange : Color.primary)
            TextField("Description", text: $log.description)
            if log.source == .git, log.estimated != log.duration {
                Text("est. \(DurationFormat.string(log.estimated))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .help("Raw estimate before scaling the day to its target")
            }
            Picker("Status", selection: $log.status) {
                ForEach(WorklogStatus.allCases, id: \.self) { Text($0.rawValue.capitalized).tag($0) }
            }
            .labelsHidden()
            .frame(width: 100)
            .disabled(log.status == .exported && log.tempoId != nil)
            Button {
                log.locked.toggle()
            } label: {
                Image(systemName: log.locked ? "lock.fill" : "lock.open")
            }
            .buttonStyle(.borderless)
            .help(log.locked ? "Locked: kept when drafts are rebuilt" : "Unlocked: replaced when drafts are rebuilt")
            Button(role: .destructive) {
                state.delete(log)
            } label: {
                Image(systemName: "trash")
            }
            .buttonStyle(.borderless)
        }
        .onAppear(perform: syncTexts)
        .onChange(of: focus) { old, _ in
            if old == .start { applyStart() }
            if old == .duration { applyDuration() }
        }
        .onChange(of: log) { old, new in
            guard old.id == new.id else { syncTexts(); return }
            // Editing an entry locks it so a rebuild won't overwrite the correction.
            if !old.sameContent(as: new) && !new.locked { log.locked = true }
            state.markDirty(new.id)
            if focus == nil { syncTexts() }
        }
    }

    private func syncTexts() {
        startText = DayKey.hm(log.start)
        durationText = DurationFormat.string(log.duration)
    }

    private func applyStart() {
        if let d = DayKey.at(log.day, hm: startText), d != log.start { log.start = d }
        startText = DayKey.hm(log.start)
    }

    private func applyDuration() {
        if let s = DurationFormat.parse(durationText), s != log.duration { log.duration = s }
        durationText = DurationFormat.string(log.duration)
    }
}

#Preview {
    ReviewView()
        .environment(AppState.preview)
        .frame(width: 900, height: 600)
}
