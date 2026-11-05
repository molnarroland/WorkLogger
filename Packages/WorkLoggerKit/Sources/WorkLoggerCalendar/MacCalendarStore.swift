import AppKit
import EventKit
import Foundation
import SwiftUI
import WorkLoggerCore

public struct MacCalendarInfo: Identifiable, Hashable {
    public var id: String  // EKCalendar.calendarIdentifier
    public var title: String
    public var account: String  // e.g. the Google account's e-mail address
    public var color: Color

    public init(id: String, title: String, account: String, color: Color) {
        self.id = id
        self.title = title
        self.account = account
        self.color = color
    }
}

/// Reads calendars through macOS (EventKit). Google accounts added in System Settings → Internet Accounts
/// show up here, so no Google Cloud project or OAuth client ID is needed.
/// Used from the main actor only (the app state and the calendar settings).
@MainActor
public final class MacCalendarStore {
    public init() {}
    public enum Access: Equatable { case notDetermined, granted, denied, writeOnly }

    private var store = EKEventStore()

    public var access: Access {
        switch EKEventStore.authorizationStatus(for: .event) {
        case .fullAccess, .authorized: return .granted
        case .notDetermined: return .notDetermined
        case .writeOnly: return .writeOnly
        case .denied, .restricted: return .denied
        @unknown default: return .denied
        }
    }

    /// Shows the system prompt once; later answers come from System Settings → Privacy → Calendars.
    public func requestAccess() async throws -> Bool {
        let granted = try await store.requestFullAccessToEvents()
        if granted { store = EKEventStore() }  // a fresh store sees the calendars right away
        return granted
    }

    public func calendars() -> [MacCalendarInfo] {
        store.calendars(for: .event)
            .map {
                MacCalendarInfo(
                    id: $0.calendarIdentifier, title: $0.title,
                    account: $0.source?.title ?? "On My Mac", color: Color(cgColor: $0.cgColor))
            }
            .sorted { ($0.account, $0.title) < ($1.account, $1.title) }
    }

    /// Timed events of the chosen calendars, recurring ones expanded. All-day, cancelled and declined events are skipped.
    public func meetings(calendarIds: Set<String>, from: Date, to: Date) -> [Meeting] {
        let calendars = store.calendars(for: .event).filter { calendarIds.contains($0.calendarIdentifier) }
        guard !calendars.isEmpty else { return [] }
        let predicate = store.predicateForEvents(withStart: from, end: to, calendars: calendars)
        return store.events(matching: predicate).compactMap { e -> Meeting? in
            guard !e.isAllDay, e.status != .canceled,
                let start = e.startDate, let end = e.endDate, end > start
            else { return nil }
            if let me = e.attendees?.first(where: { $0.isCurrentUser }), me.participantStatus == .declined { return nil }
            // Occurrences of a recurring event share an identifier, so the start time makes each one unique.
            let base = e.eventIdentifier ?? e.calendarItemIdentifier
            return Meeting(
                id: "ek:\(base)@\(Int(start.timeIntervalSince1970))",
                calendarId: e.calendar.calendarIdentifier, title: e.title ?? "(no title)",
                details: e.notes ?? "", start: start, end: end, skipped: false)
        }
    }

    // MARK: System Settings

    /// Opens Internet Accounts, where "Add Account… → Google" runs Google's own sign-in.
    public static func openInternetAccounts() {
        let candidates = [
            "x-apple.systempreferences:com.apple.Internet-Accounts-Settings.extension",
            "x-apple.systempreferences:com.apple.preferences.internetaccounts",
        ]
        for s in candidates {
            if let url = URL(string: s), NSWorkspace.shared.open(url) { return }
        }
    }

    public static func openPrivacySettings() {
        if let url = URL(string: "x-apple.systempreferences:com.apple.preference.security?Privacy_Calendars") {
            NSWorkspace.shared.open(url)
        }
    }
}
