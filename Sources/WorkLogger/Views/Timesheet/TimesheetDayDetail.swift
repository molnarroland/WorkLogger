import SwiftUI
import AppKit
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

// MARK: Detail panel

struct DayDetail: View {
    @Environment(AppState.self) private var state
    let selection: [String]
    let clear: () -> Void
    let openChats: (String) -> Void
    @State private var title = ""
    @State private var issue = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            if selection.isEmpty {
                legend
            } else if selection.count == 1, let day = selection.first {
                dayEntries(day)
            } else {
                periodForm
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .glassEffect(.regular, in: .rect(cornerRadius: 20))
    }

    private var legend: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Timesheet").font(.headline)
            Text("Click a day to see its entries. Shift-click a second day to select a stretch and book it to a project period. Drag a line from a card (or an entry from this panel) onto a workday to log it there.")
                .font(.callout).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
            ForEach([(Color.green, "Complete"), (.orange, "Partly logged"), (.red, "Nothing logged"),
                     (.purple, "Leave or holiday"), (.blue, "Logged on a rest day"), (.gray, "Rest day or upcoming")], id: \.1) { c, t in
                Label { Text(t) } icon: { Circle().fill(c).frame(width: 10, height: 10) }
            }
            .font(.callout)
            Text("Bar colors").font(.subheadline.weight(.semibold)).padding(.top, 4)
            ForEach([(WorklogSource.git.color, "Coding"), (WorklogSource.meeting.color, "Meetings"), (WorklogSource.chat.color, "Chats"),
                     (WorklogSource.absence.color, "Leave"), (Color.orange, "Needs a ticket")], id: \.1) { c, t in
                Label { Text(t) } icon: { RoundedRectangle(cornerRadius: 2).fill(c).frame(width: 10, height: 10) }
            }
            .font(.callout)
        }
    }

    private func dayEntries(_ day: String) -> some View {
        let logs = state.worklogs.filter { $0.day == day }.sorted { $0.start < $1.start }
        let logged = logs.totalDuration
        let expected = state.expected(day: day)
        return VStack(alignment: .leading, spacing: 10) {
            VStack(alignment: .leading, spacing: 2) {
                Text(DayKey.title(day)).font(.headline)
                if let label = state.dayRecords[day]?.label, !label.isEmpty {
                    Text(label).font(.callout).foregroundStyle(.secondary)
                }
                Text("\(DurationFormat.string(logged)) of \(DurationFormat.string(expected))")
                    .monospacedDigit()
                    .foregroundStyle(logged >= expected ? Color.green : Color.orange)
            }
            if logs.isEmpty {
                Text("Nothing logged yet. Collect, rebuild drafts, or add an entry.")
                    .font(.callout).foregroundStyle(.secondary)
            }
            if !logs.isEmpty || state.chats.contains(where: { $0.day == day }) {
                ScrollView {
                    VStack(alignment: .leading, spacing: 8) {
                        ChatDaySummary(day: day, open: openChats)
                        ForEach(logs) { w in
                            HStack(alignment: .top, spacing: 8) {
                                Image(systemName: "line.3.horizontal").foregroundStyle(.tertiary).imageScale(.small)
                                    .help("Drag onto a day in the calendar to move this entry")
                                Image(systemName: w.source.icon).foregroundStyle(.secondary).frame(width: 16)
                                VStack(alignment: .leading, spacing: 2) {
                                    HStack {
                                        Text(w.issueKey ?? "No issue").bold()
                                            .foregroundStyle(w.issueKey == nil ? Color.orange : Color.primary)
                                        Spacer()
                                        Text("\(DayKey.hm(w.start)) · \(DurationFormat.string(w.duration))")
                                            .monospacedDigit().foregroundStyle(.secondary)
                                    }
                                    if !w.description.isEmpty {
                                        Text(w.description).font(.callout).foregroundStyle(.secondary).lineLimit(3)
                                    }
                                }
                            }
                            .contentShape(.rect)
                            .draggable(WorklogDrag.payload([w])) {
                                Label("\(w.issueKey ?? "No issue") \(DurationFormat.string(w.duration))", systemImage: w.source.icon)
                                    .padding(6).glassEffect(.regular, in: .capsule)
                            }
                        }
                    }
                }
            }
            HStack {
                Button { state.addManual(day: day) } label: { Label("Add entry", systemImage: "plus") }
                    .buttonStyle(.glass)
                Button("Mark reviewed") { state.setStatus(.reviewed, forAllIn: day) }
                    .buttonStyle(.glass)
                    .disabled(logs.isEmpty)
            }
        }
    }

    private var periodForm: some View {
        let first = selection.first!, last = selection.last!
        return VStack(alignment: .leading, spacing: 10) {
            Text("\(selection.count) days selected").font(.headline)
            Text("\(DayKey.title(first)) – \(DayKey.title(last))").font(.callout).foregroundStyle(.secondary)
            Text("Book these days to a project: commit time and the rest of every workday go to its issue.")
                .font(.callout).foregroundStyle(.secondary).fixedSize(horizontal: false, vertical: true)
            TextField("Project name, e.g. Billing migration", text: $title)
            TextField("Issue, e.g. TAO-200", text: $issue)
            HStack {
                Button("Mark as period") {
                    state.settings.periods.append(ProjectPeriod(title: title, issueKey: issue.uppercased(), from: first, to: last))
                    state.rebuild()
                    title = ""
                    issue = ""
                    clear()
                }
                .buttonStyle(.glassProminent)
                .disabled(JiraKey.normalize(issue) == nil)
                Button("Cancel", action: clear).buttonStyle(.glass)
            }
        }
        .textFieldStyle(.roundedBorder)
    }
}
