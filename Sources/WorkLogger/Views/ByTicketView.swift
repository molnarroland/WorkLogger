import SwiftUI
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

/// Log in Tempo by hand: pick a ticket, then copy each day's duration and text into Tempo and tick it off.
struct ByTicketView: View {
    @Environment(AppState.self) private var state
    @Binding var pane: Pane?
    @State private var selected: String?

    var body: some View {
        let groups = TicketPlan.groups(state.worklogs)
        let current = groups.first { $0.id == selected } ?? groups.first { $0.key != nil } ?? groups.first
        let tickets = groups.filter { $0.key != nil }
        HStack(spacing: 0) {
            VStack(alignment: .leading, spacing: 0) {
                VStack(alignment: .leading, spacing: 2) {
                    Text("Tickets this month").font(.headline)
                    Text("\(tickets.reduce(0) { $0 + $1.enteredDays }) of \(tickets.reduce(0) { $0 + $1.days.count }) ticket-days entered")
                        .font(.callout).foregroundStyle(.secondary).monospacedDigit()
                }
                .padding(14)
                List(groups, selection: Binding(get: { current?.id }, set: { selected = $0 })) { g in
                    TicketRow(group: g).tag(g.id)
                }
                .listStyle(.sidebar)
            }
            .frame(width: 300)
            Divider()
            Group {
                if let current {
                    if current.key == nil { NoIssueDetail(group: current) { pane = .review } }
                    else { TicketDetail(group: current) }
                } else {
                    ContentUnavailableView("Nothing to log", systemImage: "checkmark.circle",
                                           description: Text("Collect the month first; its tickets show up here."))
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        }
    }
}

private struct TicketRow: View {
    let group: TicketGroup

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack {
                Text(group.key ?? "No issue").font(.body.monospaced().weight(.medium))
                    .foregroundStyle(group.key == nil ? Color.orange : Color.primary)
                Spacer()
                Text(DurationFormat.string(group.seconds)).monospacedDigit()
            }
            Text(group.key == nil ? "\(TicketDetail.days(group.days.count)) need a ticket" : "\(TicketDetail.days(group.days.count)) · \(group.days.first?.description ?? "")")
                .font(.caption).foregroundStyle(.secondary).lineLimit(1)
            if group.key != nil {
                ProgressView(value: Double(group.enteredDays), total: Double(max(group.days.count, 1)))
                    .tint(.green).controlSize(.small)
            }
        }
        .padding(.vertical, 3)
    }
}

private struct TicketDetail: View {
    @Environment(AppState.self) private var state
    let group: TicketGroup

    static func days(_ n: Int) -> String { n == 1 ? "1 day" : "\(n) days" }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .firstTextBaseline) {
                Text(group.key ?? "").font(.title.monospaced().weight(.medium))
                Spacer()
                CopyButton(title: "Copy key", text: group.key ?? "")
                CopyButton(title: "Copy all", text: TicketPlan.text(group), prominent: true)
            }
            HStack(spacing: 8) {
                Chip(Self.days(group.days.count))
                Chip("\(DurationFormat.string(group.seconds)) total")
                Chip("\(group.enteredDays) of \(group.days.count) entered",
                     tint: group.isDone ? .green : group.enteredDays > 0 ? .orange : nil)
            }
            Table(group.days) {
                TableColumn("In Tempo") { d in
                    Toggle("Entered", isOn: Binding(get: { d.entered }, set: { state.setEntered(d, $0) }))
                        .labelsHidden().toggleStyle(.checkbox)
                }
                .width(64)
                TableColumn("Day") { d in Text(DayKey.title(d.day)).foregroundStyle(d.entered ? .secondary : .primary) }
                    .width(min: 110, ideal: 130)
                TableColumn("Duration") { d in
                    Text(DurationFormat.string(d.seconds)).monospaced().fontWeight(.medium)
                        .strikethrough(d.entered).foregroundStyle(d.entered ? .secondary : .primary)
                }
                .width(80)
                TableColumn("Description") { d in Text(d.description).lineLimit(1).foregroundStyle(d.entered ? .secondary : .primary) }
                TableColumn("") { d in
                    HStack(spacing: 4) {
                        CopyButton(title: "Duration", text: DurationFormat.string(d.seconds), small: true)
                        CopyButton(title: "Text", text: d.description, small: true)
                    }
                }
                .width(150)
            }
            Text("Ticking a day marks its entries as in Tempo: they are locked and drafts rebuilds leave them alone.")
                .font(.caption).foregroundStyle(.secondary)
        }
        .padding(18)
    }
}

private struct NoIssueDetail: View {
    let group: TicketGroup
    let fix: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack {
                VStack(alignment: .leading, spacing: 2) {
                    Text("No issue").font(.title.monospaced().weight(.medium)).foregroundStyle(.orange)
                    Text("These entries can't go to Tempo until they have a ticket.").foregroundStyle(.secondary)
                }
                Spacer()
                Button("Fix in Review", action: fix).buttonStyle(.glassProminent)
            }
            Table(group.days) {
                TableColumn("Day") { d in Text(DayKey.title(d.day)) }.width(min: 110, ideal: 130)
                TableColumn("Duration") { d in Text(DurationFormat.string(d.seconds)).monospaced() }.width(80)
                TableColumn("Entries") { d in Text(d.description).lineLimit(1) }
            }
        }
        .padding(18)
    }
}

/// Copies text and briefly says so.
struct CopyButton: View {
    let title: String
    let text: String
    var prominent = false
    var small = false
    @State private var copied = false

    var body: some View {
        let button = Button(copied ? "Copied" : title) {
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setString(text, forType: .string)
            copied = true
            Task { try? await Task.sleep(for: .seconds(1.2)); copied = false }
        }
        .controlSize(small ? .small : .regular)
        if prominent { button.buttonStyle(.glassProminent) } else { button.buttonStyle(.glass) }
    }
}

/// "Signed in" / "Signed out" for a site WorkLogger reads from; checked when shown.
struct SignInChip: View {
    @Environment(AppState.self) private var state
    let source: WebSession.Source

    var body: some View {
        let session = WebSession.of(source)
        Group {
            switch session.signedIn {
            case true?: Chip("● signed in", tint: .green)
            case false?: Chip("signed out", tint: .orange)
            case nil: EmptyView()
            }
        }
        .task { await session.checkSignedIn(host: HRmasterCollector.normalize(state.settings.hrmasterURL)?.host()) }
    }
}

/// Small capsule label; colored when it says something needs attention or is done.
struct Chip: View {
    let text: String
    var tint: Color?
    init(_ text: String, tint: Color? = nil) { self.text = text; self.tint = tint }

    var body: some View {
        Text(text).font(.caption).monospacedDigit()
            .padding(.horizontal, 10).padding(.vertical, 3)
            .foregroundStyle(tint ?? .secondary)
            .glassEffect(.regular.tint((tint ?? .clear).opacity(0.15)), in: .capsule)
    }
}
