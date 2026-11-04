import Foundation
import Testing
@testable import WorkLogger

@Suite struct AppSettingsTests {
    @Test func missingFieldsFallBackToDefaults() throws {
        let s = try #require(AppSettings.decodeMerging(Data(#"{"fixedDailyHours": 6, "jiraEmail": "me@x.hu"}"#.utf8)))
        #expect(s.fixedDailyHours == 6)
        #expect(s.jiraEmail == "me@x.hu")
        #expect(s.granularityMinutes == AppSettings().granularityMinutes)
        #expect(s.tempoBaseURL == AppSettings().tempoBaseURL)
    }

    @Test func roundTrip() throws {
        var s = AppSettings()
        s.authorEmails = ["me@example.com"]
        s.periods = [ProjectPeriod(title: "Migration", issueKey: "TAO-200", from: "2026-03-02", to: "2026-03-13")]
        s.meetingRules = [MeetingRule(contains: "standup", issueKey: "INT-12")]
        let data = try JSONEncoder().encode(s)
        #expect(AppSettings.decodeMerging(data) == s)
    }

    @Test func legacyRoundingModeStillDecodes() {
        #expect(AppSettings.decodeMerging(Data(#"{"roundingMode": "sessions"}"#.utf8))?.roundingMode == .sessions)
    }

    @Test(arguments: ["not json", "[1, 2]", #"{"fixedDailyHours": "eight"}"#])
    func invalidDataGivesNil(json: String) {
        #expect(AppSettings.decodeMerging(Data(json.utf8)) == nil)
    }

    @Test func periodsAreInclusive() {
        let p = ProjectPeriod(title: "", issueKey: "", from: "2026-03-02", to: "2026-03-06")
        #expect(p.contains("2026-03-02"))
        #expect(p.contains("2026-03-06"))
        #expect(!p.contains("2026-03-07"))
        #expect(!p.contains("2026-03-01"))
    }
}
