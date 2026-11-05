import Foundation
import Testing
@testable import WorkLoggerCalendar
import WorkLoggerTestSupport
import WorkLoggerCore

@Suite struct GoogleCalendarClientTests {
    init() { Fixture.pinTimeZone() }

    static let page1 = """
    {"nextPageToken": "p2", "items": [
      {"id": "standup", "summary": "Standup", "description": "TAO-1",
       "start": {"dateTime": "2026-03-03T09:00:00+01:00"}, "end": {"dateTime": "2026-03-03T09:15:00+01:00"}},
      {"id": "allday", "summary": "Holiday", "start": {"date": "2026-03-03"}, "end": {"date": "2026-03-04"}},
      {"id": "cancelled", "status": "cancelled",
       "start": {"dateTime": "2026-03-03T10:00:00+01:00"}, "end": {"dateTime": "2026-03-03T11:00:00+01:00"}},
      {"id": "ooo", "eventType": "outOfOffice",
       "start": {"dateTime": "2026-03-03T12:00:00+01:00"}, "end": {"dateTime": "2026-03-03T13:00:00+01:00"}}
    ]}
    """
    static let page2 = """
    {"items": [
      {"id": "declined", "attendees": [{"self": true, "responseStatus": "declined"}],
       "start": {"dateTime": "2026-03-03T14:00:00+01:00"}, "end": {"dateTime": "2026-03-03T15:00:00+01:00"}},
      {"id": "review", "attendees": [{"self": true, "responseStatus": "accepted"}],
       "start": {"dateTime": "2026-03-03T16:00:00+01:00"}, "end": {"dateTime": "2026-03-03T17:00:00+01:00"}}
    ]}
    """

    func makeHTTP() -> FakeHTTP {
        FakeHTTP { r in
            if r.url?.host == "oauth2.googleapis.com" { return (200, #"{"access_token": "at", "expires_in": 3600}"#) }
            let paged = r.url?.query?.contains("pageToken=p2") ?? false
            return (200, paged ? Self.page2 : Self.page1)
        }
    }

    @Test func readsTimedEventsAcrossPages() async throws {
        let http = makeHTTP()
        let auth = GoogleAuth(clientId: "id", clientSecret: "secret",
                              secrets: InMemorySecrets([SecretKey.googleRefresh: "refresh"]), http: http)
        let (from, to) = DayKey.bounds("2026-03-03")

        let meetings = try await auth.meetings(calendarId: "me@example.com", from: from, to: to)

        #expect(meetings.map(\.id) == ["standup", "review"])
        #expect(meetings.first?.title == "Standup")
        #expect(meetings.first?.details == "TAO-1")
        #expect(meetings.first?.calendarId == "me@example.com")
        #expect(meetings.first.map { DayKey.hm($0.start) } == "09:00")
        #expect(meetings.last?.title == "(no title)")

        let requests = http.requests
        #expect(requests.count == 3)   // token refresh + two pages
        #expect(requests[1].value(forHTTPHeaderField: "Authorization") == "Bearer at")
        #expect(requests[1].url?.path == "/calendar/v3/calendars/me@example.com/events")
    }

    @Test func notSignedInWithoutARefreshToken() async throws {
        let auth = GoogleAuth(clientId: "id", clientSecret: "secret", secrets: InMemorySecrets(), http: makeHTTP())
        let (from, to) = DayKey.bounds("2026-03-03")
        await #expect(throws: GoogleAuthError.self) { try await auth.meetings(calendarId: "primary", from: from, to: to) }
    }

    @Test func signOutForgetsTheRefreshToken() {
        let secrets = InMemorySecrets([SecretKey.googleRefresh: "refresh"])
        GoogleAuth(clientId: "id", clientSecret: "secret", secrets: secrets, http: makeHTTP()).signOut()
        #expect(secrets.get(SecretKey.googleRefresh) == nil)
    }
}
