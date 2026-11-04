import Foundation

struct GoogleCalendarClient {
    let auth: GoogleAuth
    let http: any HTTPClient

    private struct EventsPage: Decodable {
        let items: [Event]?
        let nextPageToken: String?
    }
    private struct Event: Decodable {
        let id: String
        let summary: String?
        let description: String?
        let status: String?
        let eventType: String?
        let start: EventTime?
        let end: EventTime?
        let attendees: [Attendee]?
    }
    private struct EventTime: Decodable {
        let dateTime: Date?
        let date: String?
    }
    private struct Attendee: Decodable {
        let isSelf: Bool?
        let responseStatus: String?
        enum CodingKeys: String, CodingKey {
            case isSelf = "self"
            case responseStatus
        }
    }

    /// Timed events in [from, to). Skips all-day events, cancelled events, events you declined,
    /// and out-of-office / focus-time / working-location blocks.
    func meetings(calendarId: String, from: Date, to: Date) async throws -> [Meeting] {
        let token = try await auth.validAccessToken()
        let iso = ISO8601DateFormatter()
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        var allowed = CharacterSet.urlPathAllowed
        allowed.remove(charactersIn: "/#?")
        let encodedId = calendarId.addingPercentEncoding(withAllowedCharacters: allowed) ?? calendarId

        var result: [Meeting] = []
        var pageToken: String?
        repeat {
            var c = URLComponents(string: "https://www.googleapis.com/calendar/v3/calendars/\(encodedId)/events")!
            var query = [
                URLQueryItem(name: "timeMin", value: iso.string(from: from)),
                URLQueryItem(name: "timeMax", value: iso.string(from: to)),
                URLQueryItem(name: "singleEvents", value: "true"),
                URLQueryItem(name: "orderBy", value: "startTime"),
                URLQueryItem(name: "maxResults", value: "2500"),
            ]
            if let pageToken { query.append(URLQueryItem(name: "pageToken", value: pageToken)) }
            c.queryItems = query

            var req = URLRequest(url: c.url!)
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
            let (data, resp) = try await http.send(req)
            guard (resp as? HTTPURLResponse)?.statusCode == 200 else {
                throw GoogleAuthError.badResponse(String(decoding: data, as: UTF8.self))
            }
            let page = try decoder.decode(EventsPage.self, from: data)

            for e in page.items ?? [] {
                guard e.status != "cancelled", let start = e.start?.dateTime, let end = e.end?.dateTime else { continue }
                if let type = e.eventType, ["workingLocation", "outOfOffice", "focusTime", "birthday"].contains(type) { continue }
                if e.attendees?.first(where: { $0.isSelf == true })?.responseStatus == "declined" { continue }
                result.append(Meeting(id: e.id, calendarId: calendarId, title: e.summary ?? "(no title)",
                                      details: e.description ?? "", start: start, end: end, skipped: false))
            }
            pageToken = page.nextPageToken
        } while pageToken != nil
        return result
    }
}
