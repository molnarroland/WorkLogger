import SwiftUI

/// Work sessions of the range, read from HRmaster (Fetch range): one strip per day with net and reported hours.
struct SessionsView: View {
    @Environment(AppState.self) private var state
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        @Bindable var state = state
        VStack(spacing: 0) {
            HStack {
                Text("Work sessions").font(.title2.weight(.semibold))
                Spacer()
                TextField("HRmaster address", text: $state.settings.hrmasterURL, prompt: Text("https://company.hrmaster.hu"))
                    .textFieldStyle(.roundedBorder).frame(width: 240)
                    .help("Your company's HRmaster address; it differs per company")
                SignInChip(source: .hrmaster)
                Button("HRmaster…") { openWindow(id: WebSession.Source.hrmaster.windowID) }
                    .buttonStyle(.glass)
                    .disabled(HRmasterCollector.normalize(state.settings.hrmasterURL) == nil)
                    .help("Opens HRmaster inside WorkLogger; sign in there once and the login is kept")
                Button {
                    Task { await state.fetchHRmaster() }
                } label: { Label("Fetch range", systemImage: "arrow.down.circle") }
                    .buttonStyle(.glassProminent)
                    .disabled(state.isBusy || HRmasterCollector.normalize(state.settings.hrmasterURL) == nil)
                    .help("Reads the Munkaidő calendar of each month in the selected dates")
                Button("Clear sessions in range", role: .destructive) { state.clearSessionsInRange() }
                    .buttonStyle(.glass)
                    .disabled(state.sessions.isEmpty && state.dayRecords.isEmpty)
            }
            .padding(12)
            Divider()

            if state.sessions.isEmpty && state.dayRecords.isEmpty {
                ContentUnavailableView("No work sessions in this range", systemImage: "clock",
                                       description: Text("Choose Fetch range to read them from HRmaster. Without sessions the default workday from Settings is used."))
                    .frame(maxHeight: .infinity)
            } else {
                SessionList(sessions: state.sessions, records: state.dayRecords)
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
                let work = items.workIntervals
                let net = Int(Intervals.total(items.netWork))
                Section {
                    // One strip across the working hours: a missing afternoon or a long lunch stands out at a glance.
                    DayStrip(sessions: items)
                        .help(items.map { "\($0.kind == .work ? "Work" : "Break") \(DayKey.hm($0.start))–\(DayKey.hm($0.end))" }
                            .joined(separator: "\n"))
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

/// A day's sessions on a 07:00–19:00 axis (stretched when a session falls outside): work green, breaks grey.
struct DayStrip: View {
    let sessions: [WorkSession]

    var body: some View {
        let day = sessions.first.map { DayKey.startOfDay($0.start) } ?? Date()
        let hours = sessions.flatMap { [$0.start, $0.end] }.map { $0.timeIntervalSince(day) / 3600 }
        let lo = min(7, (hours.min() ?? 7).rounded(.down)), hi = max(19, (hours.max() ?? 19).rounded(.up))
        VStack(alignment: .leading, spacing: 2) {
            GeometryReader { geo in
                let x = { (d: Date) in geo.size.width * CGFloat((d.timeIntervalSince(day) / 3600 - lo) / (hi - lo)) }
                ZStack(alignment: .leading) {
                    Capsule().fill(.quaternary)
                    ForEach(sessions.sorted { ($0.kind == .work ? 0 : 1) < ($1.kind == .work ? 0 : 1) }, id: \.self) { s in
                        Capsule().fill(s.kind == .work ? Color.green.opacity(0.75) : Color.secondary.opacity(0.6))
                            .frame(width: max(2, x(s.end) - x(s.start)), height: s.kind == .work ? 14 : 8)
                            .offset(x: x(s.start))
                    }
                }
            }
            .frame(height: 16)
            HStack {
                ForEach(Array(stride(from: Int(lo), through: Int(hi), by: 2)), id: \.self) { h in
                    Text("\(h):00").font(.caption2).foregroundStyle(.tertiary)
                    if h + 2 <= Int(hi) { Spacer(minLength: 0) }
                }
            }
        }
        .padding(.vertical, 2)
    }
}

#Preview {
    SessionsView()
        .environment(AppState.preview)
        .frame(width: 900, height: 600)
}
