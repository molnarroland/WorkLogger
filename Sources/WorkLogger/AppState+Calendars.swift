import Foundation
import SwiftUI
import UniformTypeIdentifiers
import EventKit
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

/// Calendars: macOS accounts through EventKit, the optional Google Calendar API, and meetings.
extension AppState {
    // MARK: Google Calendar

    private func google() -> any GoogleCalendarAccount {
        let secret = services.secrets.get(SecretKey.googleSecret) ?? ""
        if let a = googleAccount, a.clientId == settings.googleClientId, a.clientSecret == secret { return a }
        let a = services.makeGoogle(settings.googleClientId, secret)
        googleAccount = a
        return a
    }

    func connectGoogle() async {
        isBusy = true
        defer { isBusy = false }
        status = "Finish signing in to Google in your browser…"
        do {
            try await google().signIn()
            googleConnected = true
            status = "Google Calendar connected."
        } catch {
            status = error.localizedDescription
        }
    }

    func disconnectGoogle() {
        google().signOut()
        googleConnected = false
        status = "Google Calendar disconnected."
    }

    // MARK: macOS calendars (any online account via Internet Accounts)

    var canReadMeetings: Bool {
        settings.modules.contains(.calendar)
            && (googleConnected || (calendarAccess == .granted && !settings.macCalendarIds.isEmpty))
    }

    private var googleApiCalendarId: String {
        settings.googleCalendarId.isEmpty ? "primary" : settings.googleCalendarId
    }

    func refreshCalendars() {
        calendarAccess = services.calendars.access
        macCalendars = calendarAccess == .granted ? services.calendars.calendars() : []
    }

    func openInternetAccounts() {
        MacCalendarStore.openInternetAccounts()
        status = "In Internet Accounts choose Add Account…, sign in and keep Calendars switched on."
    }

    func requestCalendarAccess() async {
        do {
            let granted = try await services.calendars.requestAccess()
            refreshCalendars()
            status = granted ? "Calendar access allowed. Tick the calendars to read."
                             : "Calendar access was not allowed. You can change this in System Settings → Privacy & Security → Calendars."
        } catch {
            status = "Calendar access failed: \(error.localizedDescription)"
        }
    }

    func isCalendarSelected(_ id: String) -> Bool { settings.macCalendarIds.contains(id) }

    func setCalendar(_ id: String, selected: Bool) {
        if selected {
            if !settings.macCalendarIds.contains(id) { settings.macCalendarIds.append(id) }
        } else {
            settings.macCalendarIds.removeAll { $0 == id }
        }
    }

    /// Reads meetings from the selected macOS calendars and, if connected, the Google Calendar API.
    @discardableResult
    func fetchMeetings() async throws -> Int {
        let macIds = calendarAccess == .granted ? Set(settings.macCalendarIds) : []
        var active = macIds
        if googleConnected { active.insert(googleApiCalendarId) }

        var count = 0
        let macEvents = services.calendars.meetings(calendarIds: macIds, from: rangeStart, to: rangeEnd)
        try db.syncMeetings(from: rangeStart, to: rangeEnd, calendarIds: macIds, activeCalendarIds: active, events: macEvents)
        count += macEvents.count

        if googleConnected {
            let events = try await google().meetings(calendarId: googleApiCalendarId, from: rangeStart, to: rangeEnd)
            try db.syncMeetings(from: rangeStart, to: rangeEnd, calendarIds: [googleApiCalendarId],
                                activeCalendarIds: active, events: events)
            count += events.count
        }
        meetings = try db.meetings(from: rangeStart, to: rangeEnd)
        return count
    }

    func refreshMeetings() async {
        isBusy = true
        defer { isBusy = false }
        do {
            let n = try await fetchMeetings()
            status = "Read \(n) meetings. Rebuild drafts to apply them."
        } catch {
            status = "Calendar: \(error.localizedDescription)"
        }
    }

    func setSkipped(_ meeting: Meeting, _ skipped: Bool) {
        attempt("Saving meeting") { try db.setMeetingSkipped(meeting.id, skipped) }
        if let i = meetings.firstIndex(where: { $0.id == meeting.id }) { meetings[i].skipped = skipped }
    }
}
