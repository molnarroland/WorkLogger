import AppKit
import SwiftUI
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

// MARK: Day cell

struct DayCell: View {
    @Environment(AppState.self) private var state
    let day: String
    let inRange: Bool
    let selected: Bool
    @State private var dropTargeted = false

    var body: some View {
        // Only the selected range is loaded, so days of the neighbouring months show just their date.
        if inRange {
            cell
        } else {
            Text(DayKey.date(day).formatted(.dateTime.day()))
                .font(.headline).foregroundStyle(.tertiary)
                .padding(8)
                .frame(maxWidth: .infinity, minHeight: 64, maxHeight: .infinity, alignment: .topLeading)
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

        // The content hangs over a flexible base, so a card never asks for more height than its week row gets:
        // the weeks always share the grid evenly and the last one stays above the status bar.
        Color.clear.overlay(alignment: .topLeading) {
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
                // The full card when the row is tall and wide enough; otherwise (a short or half-screen window) just the
                // bar and the totals, and the day panel has the rest.
                ViewThatFits {
                    VStack(alignment: .leading, spacing: 4) {
                        details(logs: logs, logged: logged, period: period)
                        Spacer(minLength: 0)
                        footer(logs: logs, logged: logged, expected: expected)
                    }
                    VStack(alignment: .leading, spacing: 4) {
                        Spacer(minLength: 0)
                        footer(logs: logs, logged: logged, expected: expected)
                    }
                }
            }
            .padding(8)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        }
        .frame(minHeight: 56)
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
        } isTargeted: {
            dropTargeted = $0 && state.acceptsWork(day)
        }
        .contentShape(.rect)
    }

    @ViewBuilder private func details(logs: [Worklog], logged: Int, period: ProjectPeriod?) -> some View {
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
            .help("\(String(localized: source.localizedLabel)) — drag to another day to log it there")
            .contentShape(.rect)
            .draggable(WorklogDrag.payload(logs.filter { $0.source == source })) {
                Label(
                    "\(String(localized: source.localizedLabel)) \(DurationFormat.string(seconds))", systemImage: source.icon
                )
                .padding(6).glassEffect(.regular, in: .capsule)
            }
        }
        // What Tempo already has for the day: green once it covers what WorkLogger logged.
        if state.tempoConnected {
            let tempo = state.tempoSeconds(day: day)
            if tempo > 0 || logged > 0 {
                HStack(spacing: 4) {
                    Image(systemName: "checkmark.seal").frame(width: 14)
                    Spacer(minLength: 2)
                    Text("Tempo \(DurationFormat.string(tempo))").monospacedDigit()
                }
                .font(.caption2)
                .lineLimit(1)
                .foregroundStyle(tempo > 0 && tempo >= logged - 60 ? Color.green : Color.secondary)
                .help("Already logged in Tempo for this day")
            }
        }
    }

    @ViewBuilder private func footer(logs: [Worklog], logged: Int, expected: Int) -> some View {
        if expected > 0 || logged > 0 {
            SourceStack(logs: logs, scale: expected)
            ViewThatFits(in: .horizontal) {
                if expected > 0 {
                    Text("\(DurationFormat.string(logged)) / \(DurationFormat.string(expected))")
                }
                Text(DurationFormat.string(logged))
            }
            .font(.caption2).monospacedDigit().foregroundStyle(.secondary).lineLimit(1)
        }
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
