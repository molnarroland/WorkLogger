import SwiftUI
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

struct ConnectSection: View {
    @Environment(AppState.self) private var state

    var body: some View {
        let accounts = Set(state.macCalendars.map(\.account)).sorted()
        Section {
            HStack(alignment: .top, spacing: 12) {
                Image(systemName: "1.circle.fill").font(.title2).foregroundStyle(.secondary)
                VStack(alignment: .leading, spacing: 6) {
                    Text("Add your calendar account in macOS")
                    Text(
                        "Add Account…, sign in, keep Calendars switched on. Any account that syncs calendars to macOS works: Google, Microsoft Exchange / Outlook, iCloud, Yahoo or CalDAV."
                    )
                    .font(.callout).foregroundStyle(.secondary)
                    Button {
                        state.openInternetAccounts()
                    } label: {
                        Label("Open Internet Accounts…", systemImage: "person.crop.circle.badge.plus")
                    }
                    .buttonStyle(.borderedProminent)
                }
            }
            HStack(alignment: .top, spacing: 12) {
                Image(systemName: "2.circle.fill").font(.title2).foregroundStyle(.secondary)
                VStack(alignment: .leading, spacing: 6) {
                    switch state.calendarAccess {
                    case .granted:
                        Label("WorkLogger can read your calendars", systemImage: "checkmark.circle.fill")
                            .foregroundStyle(.green)
                        if !accounts.isEmpty {
                            Text("Accounts: \(accounts.joined(separator: ", "))")
                                .font(.callout).foregroundStyle(.secondary)
                        }
                    case .notDetermined:
                        Text("Let WorkLogger read your calendars")
                        Button("Allow calendar access") { Task { await state.requestCalendarAccess() } }
                    case .denied, .writeOnly:
                        Text("Calendar access is turned off for WorkLogger")
                        Text("Turn on full access under Privacy & Security → Calendars, then come back.")
                            .font(.callout).foregroundStyle(.secondary)
                        HStack {
                            Button("Open Privacy settings") { MacCalendarStore.openPrivacySettings() }
                            Button("Check again") { state.refreshCalendars() }
                        }
                    }
                }
            }
        } header: {
            Text("Online Accounts")
        } footer: {
            Text("Read-only. Your password stays with macOS; WorkLogger never sees it.")
        }
    }
}

struct CalendarPicker: View {
    @Environment(AppState.self) private var state

    var body: some View {
        let byAccount = Dictionary(grouping: state.macCalendars, by: \.account)
        Section {
            if state.macCalendars.isEmpty {
                Text("No calendars yet. A newly added account can take a minute to sync.")
                    .foregroundStyle(.secondary)
            }
            ForEach(byAccount.keys.sorted(), id: \.self) { account in
                Text(account).font(.caption).foregroundStyle(.secondary)
                ForEach(byAccount[account] ?? []) { cal in
                    Toggle(
                        isOn: Binding(
                            get: { state.isCalendarSelected(cal.id) },
                            set: { state.setCalendar(cal.id, selected: $0) })
                    ) {
                        HStack(spacing: 8) {
                            Circle().fill(cal.color).frame(width: 10, height: 10)
                            Text(cal.title)
                        }
                    }
                }
            }
        } header: {
            HStack {
                Text("Calendars to read")
                Spacer()
                Button("Refresh") { state.refreshCalendars() }.buttonStyle(.borderless)
            }
        }
    }
}
