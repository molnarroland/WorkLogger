import Foundation
import Testing

@testable import WorkLogger

/// The String Catalog ships English (with singular forms) and Hungarian in the app bundle.
@Suite struct LocalizationTests {
    #if !SWIFT_PACKAGE  // `swift test` keeps resources in a separate bundle; the Xcode-hosted run checks the app
        func table(_ language: String) throws -> Bundle {
            let path = try #require(Bundle.main.path(forResource: language, ofType: "lproj"))
            return try #require(Bundle(path: path))
        }

        @Test func hungarianIsTranslated() throws {
            let hu = try table("hu")
            #expect(hu.localizedString(forKey: "Log in Tempo", value: nil, table: nil) == "Könyvelés Tempóban")
            #expect(
                String(format: hu.localizedString(forKey: "Moved %lld entries to %@.", value: nil, table: nil), 3, "kedd")
                    == "3 bejegyzés áthelyezve ide: kedd.")
        }

        @Test func englishUsesSingularForOne() throws {
            let en = try table("en")
            let format = en.localizedString(forKey: "%lld days", value: nil, table: nil)
            #expect(String.localizedStringWithFormat(format, 1) == "1 day")
            #expect(String.localizedStringWithFormat(format, 3) == "3 days")
        }
    #endif
}
