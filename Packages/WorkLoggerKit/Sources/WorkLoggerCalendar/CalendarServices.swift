import Foundation
import WorkLoggerCore

/// Calendars the system knows about. The app reads them through EventKit (`MacCalendarStore`).
@MainActor
public protocol CalendarSource: AnyObject {
    var access: MacCalendarStore.Access { get }
    func requestAccess() async throws -> Bool
    func calendars() -> [MacCalendarInfo]
    func meetings(calendarIds: Set<String>, from: Date, to: Date) -> [Meeting]
}

extension MacCalendarStore: CalendarSource {}

/// A Google account read through the Calendar API (`GoogleAuth`).
@MainActor
public protocol GoogleCalendarAccount: AnyObject {
    var clientId: String { get }
    var clientSecret: String { get }
    func signIn() async throws
    func signOut()
    func meetings(calendarId: String, from: Date, to: Date) async throws -> [Meeting]
}
