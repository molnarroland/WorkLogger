import SwiftUI
import WorkLoggerCore

/// The menu bar item: today's logged time at a glance, the month's next step, and quick actions.
struct MenuBarView: View {
    @Environment(AppState.self) private var state
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        let today = DayKey.key(state.services.now())
        let logs = state.worklogs.filter { $0.day == today }
        let inRange = today >= state.fromDay && today <= state.toDay
        let progress = state.tempoProgress
        VStack(alignment: .leading, spacing: 10) {
            VStack(alignment: .leading, spacing: 4) {
                Text("Today").font(.headline)
                if inRange {
                    Text("\(DurationFormat.string(logs.totalDuration)) of \(DurationFormat.string(state.expected(day: today)))")
                        .monospacedDigit()
                    SourceStack(logs: logs, scale: state.expected(day: today), height: 6)
                } else {
                    Text("Today is outside the selected dates.").foregroundStyle(.secondary)
                }
            }
            if progress.missing > 0 || progress.total > 0 {
                VStack(alignment: .leading, spacing: 2) {
                    if progress.missing > 0 { Text("\(progress.missing) entries need a ticket").foregroundStyle(.orange) }
                    if progress.total > 0 { Text("\(progress.entered) of \(progress.total) ticket-days in Tempo").monospacedDigit() }
                }
                .font(.callout)
            }
            if state.isBusy {
                Label(state.status, systemImage: "arrow.triangle.2.circlepath").font(.caption).lineLimit(2)
            }
            Divider()
            Button("Collect range") { Task { await state.collect() } }.disabled(state.isBusy)
            if state.settings.modules.contains(.chats) {
                Button("Fetch Google Chat") { Task { await state.fetchGoogleChat() } }.disabled(state.isBusy)
            }
            Button("Open Timesheet") { show(.timesheet) }
            Button("Log in Tempo") { show(.byTicket) }
            Divider()
            Button("Quit WorkLogger") { NSApp.terminate(nil) }
        }
        .buttonStyle(.borderless)
        .padding(14)
        .frame(width: 260, alignment: .leading)
    }

    private func show(_ pane: Pane) {
        state.requestedPane = pane
        openWindow(id: "main")
        NSApp.activate()
    }
}

/// The item's title: today's logged time, or just the clock outside the selected dates.
struct MenuBarLabel: View {
    @Environment(AppState.self) private var state

    var body: some View {
        let today = DayKey.key(state.services.now())
        if today >= state.fromDay && today <= state.toDay {
            Label(DurationFormat.string(state.worklogs.filter { $0.day == today }.totalDuration), systemImage: "clock")
                .labelStyle(.titleAndIcon)
        } else {
            Image(systemName: "clock")
        }
    }
}
