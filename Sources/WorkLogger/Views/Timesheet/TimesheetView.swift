import AppKit
import SwiftUI
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

/// Tempo-style month grid: every day shows what is logged against what it should be, so gaps stand out.
/// Click a day for its entries; shift-click to select several days and mark them as a project period.
struct TimesheetView: View {
    @Environment(AppState.self) private var state
    @State private var selection: Set<String> = []
    @State private var chatDay: String?
    @State private var anchor: String?
    /// Room for the day panel beside the grid; on a half-screen window it opens as a popover on the clicked day.
    @State private var wide = true
    @State private var showPopover = false
    @State private var showLegend = false

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
                let missingDays = state.onlyMissingDays ? state.missingDays : nil
                GlassEffectContainer(spacing: 6) {
                    VStack(spacing: 6) {
                        ForEach(weeks, id: \.self) { week in
                            HStack(spacing: 6) {
                                ForEach(week, id: \.self) { day in
                                    DayCell(day: day, inRange: inRange.contains(day), selected: selection.contains(day))
                                        .opacity(missingDays.map { $0.contains(day) || !inRange.contains(day) ? 1 : 0.3 } ?? 1)
                                        .onTapGesture { select(day, in: inRange) }
                                        .popover(isPresented: popover(for: day), arrowEdge: .trailing) {
                                            detail(framed: false)
                                        }
                                }
                            }
                        }
                    }
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
                .padding(.horizontal, 16)
                .padding(.bottom, 16)
            }

            if wide {
                detail(framed: true)
                    .frame(width: 320)
                    .frame(maxHeight: .infinity, alignment: .top)
                    .padding([.top, .trailing, .bottom], 12)
            }
        }
        .onGeometryChange(for: Bool.self) {
            $0.size.width >= 900
        } action: {
            wide = $0
        }
        .chatInspector(day: $chatDay)
    }

    private func detail(framed: Bool) -> some View {
        DayDetail(
            selection: selection.sorted(), framed: framed,
            clear: {
                selection = []
                showPopover = false
            }, openChats: { chatDay = $0 })
    }

    /// The popover hangs off the last selected day (the clicked one, or the end of a shift-clicked stretch).
    private func popover(for day: String) -> Binding<Bool> {
        Binding(
            get: { !wide && showPopover && selection.max() == day },
            set: { if !$0 { showPopover = false } })
    }

    private func header(logged: Int, expected: Int) -> some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: 10) {
                monthControls
                Spacer(minLength: 8)
                summary(logged: logged, expected: expected, alignment: .trailing)
            }
            VStack(alignment: .leading, spacing: 8) {
                monthControls
                summary(logged: logged, expected: expected, alignment: .leading)
            }
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 14)
    }

    @ViewBuilder private var monthControls: some View {
        HStack(spacing: 10) {
            Button {
                state.shiftMonth(-1)
            } label: {
                Image(systemName: "chevron.left")
            }
            .buttonStyle(.glass)
            Text(state.rangeStart.formatted(.dateTime.month(.wide).year()))
                .font(.title2.weight(.semibold))
                .lineLimit(1).fixedSize()
            Button {
                state.shiftMonth(1)
            } label: {
                Image(systemName: "chevron.right")
            }
            .buttonStyle(.glass)
            Button("This month") { state.setRange(.thisMonth) }
                .buttonStyle(.glass)
            if state.tempoConnected {
                Button {
                    Task { await state.refreshTempo() }
                } label: {
                    Label("Read Tempo", systemImage: "arrow.clockwise")
                }
                .buttonStyle(.glass)
                .help("Read what is already logged in Tempo for these days")
            }
            if !wide {
                Button {
                    showLegend = true
                } label: {
                    Image(systemName: "info.circle")
                }
                .buttonStyle(.glass)
                .help("What the colors mean")
                .popover(isPresented: $showLegend, arrowEdge: .bottom) {
                    DayDetail(selection: [], framed: false, clear: {}, openChats: { _ in })
                }
            }
        }
    }

    private func summary(logged: Int, expected: Int, alignment: HorizontalAlignment) -> some View {
        VStack(alignment: alignment, spacing: 2) {
            Text("\(DurationFormat.string(logged)) of \(DurationFormat.string(expected))")
                .font(.headline).monospacedDigit()
            let gap = expected - logged
            let missing = state.worklogs.missingIssue.count
            let left =
                gap > 0
                ? String(localized: "\(DurationFormat.string(gap)) still to log") : String(localized: "Everything logged")
            Text(missing > 0 ? String(localized: "\(left) · \(missing) need a ticket") : left)
                .font(.callout).lineLimit(1)
                .foregroundStyle(gap > 0 || missing > 0 ? Color.orange : Color.green)
        }
        .fixedSize()
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
        showPopover = !selection.isEmpty
    }
}

#Preview {
    TimesheetView()
        .environment(AppState.preview)
        .frame(width: 1180, height: 760)
}

#Preview("Empty") {
    TimesheetView()
        .environment(AppState.empty)
        .frame(width: 1180, height: 760)
}
