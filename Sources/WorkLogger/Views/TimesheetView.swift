import SwiftUI
import AppKit

/// Tempo-style month grid: every day shows what is logged against what it should be, so gaps stand out.
/// Click a day for its entries; shift-click to select several days and mark them as a project period.
struct TimesheetView: View {
    @Environment(AppState.self) private var state
    @State private var selection: Set<String> = []
    @State private var chatDay: String?
    @State private var anchor: String?

    private var weeks: [[String]] {
        let cal = DayKey.calendar
        guard let first = cal.dateInterval(of: .weekOfYear, for: state.rangeStart)?.start,
              let last = cal.dateInterval(of: .weekOfYear, for: state.rangeEnd.addingTimeInterval(-1))?.end
        else { return [] }
        let days = DayKey.days(from: first, to: last.addingTimeInterval(-1))
        return stride(from: 0, to: days.count, by: 7).map { Array(days[$0..<min($0 + 7, days.count)]) }
    }

    var body: some View {
        let weeks = weeks
        let inRange = Set(DayKey.days(from: state.from, to: state.to))
        let logged = state.worklogs.totalDuration
        let expected = inRange.reduce(0) { $0 + state.expected(day: $1) }

        HStack(spacing: 0) {
            VStack(spacing: 0) {
                header(logged: logged, expected: expected)
                weekdayRow(weeks.first ?? [])
                GlassEffectContainer(spacing: 6) {
                    VStack(spacing: 6) {
                        ForEach(weeks, id: \.self) { week in
                            HStack(spacing: 6) {
                                ForEach(week, id: \.self) { day in
                                    DayCell(day: day, inRange: inRange.contains(day), selected: selection.contains(day))
                                        .onTapGesture { select(day, in: inRange) }
                                }
                            }
                        }
                    }
                }
                .padding(.horizontal, 16)
                .padding(.bottom, 16)
                Spacer(minLength: 0)
            }
            .frame(minWidth: 640)

            DayDetail(selection: selection.sorted(), clear: { selection = [] }, openChats: { chatDay = $0 })
                .frame(width: 320)
                .frame(maxHeight: .infinity, alignment: .top)
                .padding([.top, .trailing, .bottom], 12)
        }
        .chatInspector(day: $chatDay)
    }

    private func header(logged: Int, expected: Int) -> some View {
        HStack(spacing: 10) {
            Button { state.shiftMonth(-1) } label: { Image(systemName: "chevron.left") }
                .buttonStyle(.glass)
            Text(state.rangeStart.formatted(.dateTime.month(.wide).year()))
                .font(.title2.weight(.semibold))
                .frame(minWidth: 170)
            Button { state.shiftMonth(1) } label: { Image(systemName: "chevron.right") }
                .buttonStyle(.glass)
            Button("This month") { state.setRange(.thisMonth) }
                .buttonStyle(.glass)
            Spacer()
            VStack(alignment: .trailing, spacing: 2) {
                Text("\(DurationFormat.string(logged)) of \(DurationFormat.string(expected))")
                    .font(.headline).monospacedDigit()
                let gap = expected - logged
                let missing = state.worklogs.missingIssue.count
                Text((gap > 0 ? "\(DurationFormat.string(gap)) still to log" : "Everything logged")
                     + (missing > 0 ? " · \(missing) need a ticket" : ""))
                    .font(.callout)
                    .foregroundStyle(gap > 0 || missing > 0 ? Color.orange : Color.green)
            }
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 14)
    }

    private func weekdayRow(_ week: [String]) -> some View {
        HStack(spacing: 6) {
            ForEach(week, id: \.self) { day in
                Text(DayKey.date(day).formatted(.dateTime.weekday(.abbreviated)))
                    .font(.caption.weight(.semibold))
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity)
            }
        }
        .padding(.horizontal, 16)
        .padding(.bottom, 6)
    }

    private func select(_ day: String, in range: Set<String>) {
        guard range.contains(day) else { return }
        let flags = NSEvent.modifierFlags
        if flags.contains(.shift), let anchor {
            let (a, b) = anchor < day ? (anchor, day) : (day, anchor)
            selection = Set(DayKey.days(from: DayKey.date(a), to: DayKey.date(b)))
        } else {
            selection = selection == [day] ? [] : [day]
            anchor = day
        }
    }
}

// MARK: Day cell

private struct DayCell: View {
    @Environment(AppState.self) private var state
    let day: String
    let inRange: Bool
    let selected: Bool
    @State private var dropTargeted = false

    var body: some View {
        // Only the selected range is loaded, so days of the neighbouring months show just their date.
        if inRange { cell } else {
            Text(DayKey.date(day).formatted(.dateTime.day()))
                .font(.headline).foregroundStyle(.tertiary)
                .padding(8)
                .frame(maxWidth: .infinity, minHeight: 104, maxHeight: .infinity, alignment: .topLeading)
                .overlay(RoundedRectangle(cornerRadius: 12).strokeBorder(.quaternary, lineWidth: 1))
                .opacity(0.5)
        }
    }

    @ViewBuilder private var cell: some View {
        let logs = state.worklogs.filter { $0.day == day }
        let logged = logs.totalDuration
        let expected = state.expected(day: day)
        let record = state.dayRecords[day]
        let isAbsence = record?.status.isAbsence ?? false
        let unresolved = logs.contains(where: \.needsIssue)
        let isFuture = day > DayKey.key(Date())
        let color = color(for: DayProgress(logged: logged, expected: expected, absence: isAbsence, future: isFuture))
        let period = state.makeBuilder().period(for: day)

        VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: 4) {
                Text(DayKey.date(day).formatted(.dateTime.day()))
                    .font(.headline)
                    .foregroundStyle(day == DayKey.key(Date()) ? Color.accentColor : Color.primary)
                if unresolved {
                    Image(systemName: "exclamationmark.triangle.fill").foregroundStyle(.orange).imageScale(.small)
                        .help("Entries without issue")
                }
                Spacer(minLength: 0)
                if let record, record.status != .workday, !record.label.isEmpty {
                    Text(record.label).font(.caption2).foregroundStyle(.purple).lineLimit(1)
                }
            }
            if let period {
                Text(period.title.isEmpty ? period.issueKey : period.title)
                    .font(.caption2.weight(.medium))
                    .lineLimit(1)
                    .padding(.horizontal, 6).padding(.vertical, 1)
                    .background(PeriodsView.color(for: period, in: state.settings.periods).opacity(0.25), in: Capsule())
            }
            // Time per kind of work (coding, meetings…) as icon + duration; issues are listed in the day panel.
            // One line per kind (a two-column grid was too narrow for the numbers); at most 3 fit the card.
            ForEach(logs.totalsBySource.prefix(3), id: \.source) { source, seconds in
                HStack(spacing: 4) {
                    Image(systemName: source.icon).foregroundStyle(.secondary).frame(width: 14)
                    Spacer(minLength: 2)
                    Text(DurationFormat.string(seconds)).monospacedDigit()
                }
                .font(.caption2)
                .lineLimit(1)
                .help("\(source.label) — drag to another day to log it there")
                .contentShape(.rect)
                .draggable(WorklogDrag.payload(logs.filter { $0.source == source })) {
                    Label("\(source.label) \(DurationFormat.string(seconds))", systemImage: source.icon)
                        .padding(6).glassEffect(.regular, in: .capsule)
                }
            }
            Spacer(minLength: 0)
            if expected > 0 || logged > 0 {
                SourceStack(logs: logs, scale: expected)
                Text(expected > 0 ? "\(DurationFormat.string(logged)) / \(DurationFormat.string(expected))" : DurationFormat.string(logged))
                    .font(.caption2).monospacedDigit().foregroundStyle(.secondary)
            }
        }
        .padding(8)
        .frame(maxWidth: .infinity, minHeight: 104, maxHeight: .infinity, alignment: .topLeading)
        .background(color.opacity(0.10), in: .rect(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).strokeBorder(color.opacity(0.45), lineWidth: 1))
        .glassEffect(selected ? .regular.tint(.accentColor.opacity(0.35)).interactive() : .identity, in: .rect(cornerRadius: 12))
        .clipShape(.rect(cornerRadius: 12))
        .overlay {
            if dropTargeted {
                RoundedRectangle(cornerRadius: 12).strokeBorder(Color.accentColor, lineWidth: 2)
            }
        }
        .dropDestination(for: String.self) { items, _ in
            let ids = items.flatMap(WorklogDrag.ids)
            guard !ids.isEmpty, state.acceptsWork(day) else { return false }
            state.move(Set(ids), to: day)
            return true
        } isTargeted: { dropTargeted = $0 && state.acceptsWork(day) }
        .contentShape(.rect)
    }

    private func color(for progress: DayProgress) -> Color {
        switch progress {
        case .absence: return .purple
        case .extra: return .blue
        case .free, .upcoming: return .gray
        case .complete: return .green
        case .empty: return .red
        case .partial: return .orange
        }
    }
}

// MARK: Detail panel

private struct DayDetail: View {
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

#Preview {
    TimesheetView()
        .environment(AppState.preview)
        .frame(width: 1180, height: 760)
}
