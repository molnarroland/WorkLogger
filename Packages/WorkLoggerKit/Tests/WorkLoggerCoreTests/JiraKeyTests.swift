import Testing
@testable import WorkLoggerCore
import WorkLoggerTestSupport

@Suite struct JiraKeyTests {
    @Test func findsFirstKey() {
        #expect(JiraKey.find(in: "Fix ABC-12 login, see ABC-13", known: []) == "ABC-12")
    }

    @Test func knownPrefixesWin() {
        let text = "UTF-8 handling, FOO-1 and TAO-2"
        #expect(JiraKey.find(in: text, known: ["TAO"]) == "TAO-2")
        #expect(JiraKey.find(in: text, known: []) == "FOO-1")   // UTF is on the deny list
    }

    @Test(arguments: ["UTF-8 and ISO-8601", "xABC-12", "ABC-12a", "abc-12", "A-1", "no keys here"])
    func ignoresNonKeys(text: String) {
        #expect(JiraKey.find(in: text, known: []) == nil)
    }

    @Test func deniedPrefixIsStillUsedWhenKnown() {
        #expect(JiraKey.find(in: "ISO-5 rollout", known: ["ISO"]) == "ISO-5")
    }

    @Test func normalize() {
        #expect(JiraKey.normalize("  tao-1 \n") == "TAO-1")
        #expect(JiraKey.normalize("   ") == nil)
        #expect(JiraKey.normalize(nil) == nil)
    }

    @Test func prefix() {
        #expect(JiraKey.prefix(of: "TAO-12") == "TAO")
    }

    @Test(arguments: [("ABC-12: Fix login", "Fix login"), ("[ABC-12] Fix login", "Fix login"),
                      ("ABC-12 - Fix login", "Fix login"), ("ABC-12 – Fix login", "Fix login"),
                      ("Fix ABC-12 login", "Fix ABC-12 login")])
    func stripLeadingKey(subject: String, expected: String) {
        #expect(JiraKey.stripLeadingKey(subject) == expected)
    }
}
