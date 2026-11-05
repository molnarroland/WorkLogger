import SwiftUI
import AppKit
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

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

#Preview {
    TimesheetView()
        .environment(AppState.preview)
        .frame(width: 1180, height: 760)
}
