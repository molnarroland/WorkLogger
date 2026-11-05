import XCTest
@testable import WorkLogger
import WorkLoggerCore
import WorkLoggerStore

final class GitScannerTests: XCTestCase {
    func testExcludedFolders() {
        let ex = ["archive", "/Users/me/Work/old"]
        XCTAssertTrue(GitScanner.isExcluded("/Users/me/Work/archive/repo", by: ex))
        XCTAssertTrue(GitScanner.isExcluded("/Users/me/Work/old", by: ex))
        XCTAssertTrue(GitScanner.isExcluded("/Users/me/Work/old/repo", by: ex))
        XCTAssertFalse(GitScanner.isExcluded("/Users/me/Work/older/repo", by: ex))
        XCTAssertFalse(GitScanner.isExcluded("/Users/me/Work/archived", by: ex))
    }
}
