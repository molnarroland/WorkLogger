import Foundation
import Testing
@testable import WorkLogger

@MainActor
@Suite struct HRmasterCollectorTests {
    @Test func addressIsReducedToTheCompanySite() {
        #expect(HRmasterCollector.normalize("flexinform.hrmaster.hu")?.absoluteString == "https://flexinform.hrmaster.hu")
        #expect(HRmasterCollector.normalize(" https://flexinform.hrmaster.hu/Datacenter/#/x ")?.absoluteString == "https://flexinform.hrmaster.hu")
        #expect(HRmasterCollector.normalize("") == nil)
        #expect(HRmasterCollector.normalize("nonsense") == nil)
    }

    @Test func workingTimePage() {
        let base = HRmasterCollector.normalize("flexinform.hrmaster.hu")!
        #expect(HRmasterCollector.workingTimeURL(base).absoluteString == "https://flexinform.hrmaster.hu/Datacenter/#/employeeWorkingTime/own")
    }

    @Test func rangeIsReadMonthByMonth() {
        let months = HRmasterCollector.months(from: Fixture.at("2026-11-20", "00:00"), to: Fixture.at("2027-01-05", "00:00"))
        #expect(months.map { "\($0.year)-\($0.month)" } == ["2026-11", "2026-12", "2027-1"])
    }
}
