import SwiftUI

/// Paste the monthly report of the work-time logger. HRmaster's "Munkaidő" page is recognised automatically;
/// other reports go through the generic parser.
struct SessionsView: View {
    @Environment(AppState.self) private var state
    @State private var raw = ""
    @State private var parsed: ParsedTimesheet?

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Text("Work sessions").font(.title2.weight(.semibold))
                Spacer()
                Button("Clear sessions in range", role: .destructive) { state.clearSessionsInRange() }
                    .buttonStyle(.glass)
                    .disabled(state.sessions.isEmpty && state.dayRecords.isEmpty)
            }
            .padding(12)
            Divider()

            HSplitView {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Paste the monthly report").font(.headline)
                    Text("In HRmaster open Munkaidő for the month, inspect the table and copy its HTML. Several months can be pasted at once.")
                        .font(.callout).foregroundStyle(.secondary)
                    TextEditor(text: $raw)
                        .font(.system(.body, design: .monospaced))
                        .border(Color.secondary.opacity(0.3))
                    HStack {
                        Button("Parse") {
                            let cal = DayKey.calendar
                            parsed = TimeloggerParser(breakKeywords: state.settings.breakKeywords,
                                                      shortDateMonthFirst: state.settings.shortDateMonthFirst,
                                                      defaultYear: cal.component(.year, from: state.from))
                                .parseTimesheet(raw, fallbackMonth: cal.component(.month, from: state.from))
                        }
                        .disabled(raw.isEmpty)
                        Spacer()
                        if let parsed, !parsed.isEmpty {
                            Button("Import \(parsed.dayKeys.count) days") {
                                state.importTimesheet(parsed)
                                self.parsed = nil
                                raw = ""
                            }
                            .buttonStyle(.borderedProminent)
                        }
                    }
                }
                .padding(12)
                .frame(minWidth: 340)

                VStack(alignment: .leading, spacing: 8) {
                    if let parsed {
                        if parsed.isEmpty {
                            ContentUnavailableView("No times found", systemImage: "questionmark.circle",
                                                   description: Text("Paste the HTML of the HRmaster Munkaidő table, or a report with dates and times like 08:00–12:00."))
                        } else {
                            Text("Preview (\(parsed.format)) — replaces the imported data of these days").font(.headline)
                            ForEach(parsed.warnings, id: \.self) { w in
                                Label(w, systemImage: "exclamationmark.triangle").foregroundStyle(.orange).font(.callout)
                            }
                            SessionList(sessions: parsed.sessions,
                                        records: Dictionary(uniqueKeysWithValues: parsed.days.map { ($0.day, $0) }))
                        }
                    } else {
                        Text("Imported for this range").font(.headline)
                        if state.sessions.isEmpty && state.dayRecords.isEmpty {
                            ContentUnavailableView("No work sessions imported", systemImage: "clock",
                                                   description: Text("Without sessions the default workday from Settings is used."))
                        } else {
                            SessionList(sessions: state.sessions, records: state.dayRecords)
                        }
                    }
                }
                .padding(12)
                .frame(minWidth: 400)
            }
        }
    }
}

struct SessionList: View {
    let sessions: [WorkSession]
    let records: [String: DayRecord]

    var body: some View {
        let byDay = Dictionary(grouping: sessions, by: \.day)
        let days = Set(byDay.keys).union(records.keys).sorted()
        List {
            ForEach(days, id: \.self) { day in
                let items = (byDay[day] ?? []).sorted { $0.start < $1.start }
                let record = records[day]
                let work = items.filter { $0.kind == .work }.map { Interval(start: $0.start, end: $0.end) }
                let pauses = items.filter { $0.kind == .pause }.map { Interval(start: $0.start, end: $0.end) }
                let net = Int(Intervals.total(Intervals.subtract(work, pauses)))
                Section {
                    ForEach(items, id: \.self) { s in
                        HStack {
                            Image(systemName: s.kind == .work ? "briefcase" : "cup.and.saucer")
                                .foregroundStyle(s.kind == .work ? Color.primary : Color.secondary)
                                .frame(width: 18)
                            Text("\(DayKey.hm(s.start)) – \(DayKey.hm(s.end))").monospacedDigit()
                            Text(DurationFormat.string(s.duration)).foregroundStyle(.secondary).monospacedDigit()
                            Spacer()
                            Text(s.label).foregroundStyle(.tertiary)
                        }
                    }
                } header: {
                    HStack(spacing: 10) {
                        Text(DayKey.title(day))
                        if let record, !record.label.isEmpty {
                            Text(record.label)
                                .foregroundStyle(record.status == .workday ? Color.secondary : Color.blue)
                        }
                        Spacer()
                        if !work.isEmpty {
                            Text("net \(DurationFormat.string(net))").monospacedDigit()
                        }
                        if let reported = record?.reportedWorked, reported > 0 {
                            Text("HR \(DurationFormat.string(reported))").monospacedDigit()
                                .foregroundStyle(!work.isEmpty && abs(reported - net) > 60 ? Color.orange : Color.secondary)
                        }
                    }
                }
            }
        }
        .listStyle(.inset)
    }
}

#Preview {
    SessionsView()
        .environment(AppState.preview)
        .frame(width: 900, height: 600)
}
