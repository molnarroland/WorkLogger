import Foundation
import Testing

@testable import WorkLoggerWeb

@Suite struct WebLocalizationTests {
    @Test func progressAndErrorsAreTranslatedToHungarian() throws {
        let path = try #require(WebResources.bundle.path(forResource: "hu", ofType: "lproj"))
        let hu = try #require(Bundle(path: path))
        #expect(hu.localizedString(forKey: "my messages", value: nil, table: nil) == "saját üzenetek")
        let format = hu.localizedString(forKey: "HRmaster: reading %@ (%lld of %lld)…", value: nil, table: nil)
        #expect(String(format: format, "2026. október", 1, 2) == "HRmaster: 2026. október olvasása (1/2)…")
    }
}
