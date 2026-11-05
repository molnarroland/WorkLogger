import Foundation

/// Kind of day as reported by the HR system.
public enum DayStatus: String, Codable, CaseIterable {
    case workday, restDay, publicHoliday, leave, other

    /// HRmaster marks days with `Type_CalendarDayStatus_<X>`; leave of any kind (Szabadság, betegszabadság…) is `LabourCode`.
    public init(hrmasterType: String) {
        switch hrmasterType {
        case "Workday": self = .workday
        case "RestDay": self = .restDay
        case "PublicHoliday": self = .publicHoliday
        case "LabourCode": self = .leave
        default: self = .other
        }
    }

    public var isAbsence: Bool { self == .leave || self == .publicHoliday }
}

/// What the HR system says about one day, independent of the individual sessions.
public struct DayRecord: Identifiable, Hashable {
    public var day: String
    public var status: DayStatus
    public var label: String            // e.g. "Munkanap (Home Office)", "Szabadság", "Pihenőnap"
    public var reportedWorked: Int?     // "Ledolgozott órák", seconds
    public var reportedBreaks: Int?     // "Napi nem fizetett szünet", seconds
    public var id: String { day }

    public init(day: String, status: DayStatus, label: String, reportedWorked: Int?, reportedBreaks: Int?) {
        self.day = day
        self.status = status
        self.label = label
        self.reportedWorked = reportedWorked
        self.reportedBreaks = reportedBreaks
    }
}

public struct ParsedTimesheet {
    public var sessions: [WorkSession] = []
    public var days: [DayRecord] = []
    public var format: String = ""
    public var warnings: [String] = []

    public var isEmpty: Bool { sessions.isEmpty && days.isEmpty }
    public var dayKeys: Set<String> { Set(days.map(\.day)).union(sessions.map(\.day)) }

    public init(sessions: [WorkSession] = [], days: [DayRecord] = [], format: String = "", warnings: [String] = []) {
        self.sessions = sessions
        self.days = days
        self.format = format
        self.warnings = warnings
    }
}

/// Parses the monthly "Munkaidő" table of HRmaster (Datacenter → Calendar) pasted as HTML.
///
/// The page is read through its `data-e2e-testing` attributes, which are stable across UI changes:
/// - `Datacenter.Calendar.PeriodSelect` → "2026. március" (year and month; several pasted months are fine)
/// - `Datacenter.Employee.Calendar.RowRepeater` → one table row per day
/// - `Datacenter.Calendar.Day` → "02." and `Type_CalendarDayStatus_*` → workday / rest day / holiday / leave
/// - `FactWorkedHoursSum`, `UnpaidRestBreaksSum` → the totals the HR system calculated
/// - `WorkScheduleRepeater` cells (Munkaidő 1–n) and `RestBreaksRepeater` cells (Szünet 1–n),
///   each holding a From and a To `Evolution.TimepickerSwitchInput`
public struct HRmasterParser {
    public var fallbackYear: Int
    public var fallbackMonth: Int

    public static func matches(_ raw: String) -> Bool {
        raw.contains("Datacenter.Employee.Calendar.RowRepeater")
    }

    private static func re(_ p: String) -> NSRegularExpression { try! NSRegularExpression(pattern: p) }

    private static let rowMarker = #"data-e2e-testing="Datacenter.Employee.Calendar.RowRepeater""#
    private static let periodRegex = re(#"data-e2e-testing="Datacenter\.Calendar\.PeriodSelect"[^>]*>\s*(\d{4})\.?\s*([^\s<]+)"#)
    private static let dayRegex = re(#"data-e2e-testing="Datacenter\.Calendar\.Day"[^>]*>\s*(\d{1,2})"#)
    private static let statusTypeRegex = re(#"Type_CalendarDayStatus_([A-Za-z]+)"#)
    private static let statusLabelRegex = re(#"data-e2e-testing="Datacenter\.Calendar\.Day\.Status\.[A-Za-z]+"[^>]*>\s*<div[^>]*>([^<]*)"#)
    private static let workedRegex = re(#"FactWorkedHoursSum"[^>]*>\s*(\d{1,3}):(\d{2})"#)
    private static let unpaidRegex = re(#"UnpaidRestBreaksSum"[^>]*>\s*(\d{1,3}):(\d{2})"#)
    private static let cellRegex = re(#"data-e2e-testing="Datacenter\.(?:Employee\.Calendar\.(WorkScheduleRepeater|RestBreaksRepeater)|Calendar\.Actual\.PaidRestBreak\.Column)""#)
    private static let pickerRegex = re(#"data-e2e-testing="Evolution\.TimepickerSwitchInput"[^>]*>([^<]*)<"#)
    private static let inputValueRegex = re(#"value="(\d{1,2}:\d{2})""#)
    private static let timeRegex = re(#"^\s*(\d{1,2}):(\d{2})(?::\d{2})?\s*$"#)

    private static let months: [String: Int] = [
        "január": 1, "február": 2, "március": 3, "április": 4, "május": 5, "június": 6,
        "július": 7, "augusztus": 8, "szeptember": 9, "október": 10, "november": 11, "december": 12,
        "january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6,
        "july": 7, "august": 8, "september": 9, "october": 10
    ]

    public static func monthNumber(_ s: String) -> Int? {
        let key = s.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: ". "))
        if let n = Int(key), (1...12).contains(n) { return n }
        return months[key]
    }

    public func parse(_ raw: String) -> ParsedTimesheet {
        var result = ParsedTimesheet(format: "HRmaster")
        let ns = raw as NSString
        let periods = Self.periodRegex.matches(in: raw, range: NSRange(location: 0, length: ns.length))

        // Split into month blocks; rows before the first period header use the fallback month.
        var blocks: [(text: String, year: Int, month: Int)] = []
        if periods.isEmpty {
            blocks = [(raw, fallbackYear, fallbackMonth)]
            result.warnings.append("No month header found; assumed \(fallbackYear)-\(String(format: "%02d", fallbackMonth)).")
        } else {
            if periods[0].range.location > 0 {
                blocks.append((ns.substring(to: periods[0].range.location), fallbackYear, fallbackMonth))
            }
            for (i, m) in periods.enumerated() {
                let start = m.range.location
                let end = i + 1 < periods.count ? periods[i + 1].range.location : ns.length
                let year = Int(ns.substring(with: m.range(at: 1))) ?? fallbackYear
                let monthText = ns.substring(with: m.range(at: 2))
                let month = Self.monthNumber(monthText) ?? fallbackMonth
                if Self.monthNumber(monthText) == nil {
                    result.warnings.append("Unknown month “\(monthText)”; assumed \(month).")
                }
                blocks.append((ns.substring(with: NSRange(location: start, length: end - start)), year, month))
            }
        }

        for block in blocks {
            for chunk in block.text.components(separatedBy: Self.rowMarker).dropFirst() {
                parseRow(chunk, year: block.year, month: block.month, into: &result)
            }
        }

        result.sessions.sort { $0.start < $1.start }
        result.days.sort { $0.day < $1.day }
        return result
    }

    // MARK: Row

    private func parseRow(_ row: String, year: Int, month: Int, into result: inout ParsedTimesheet) {
        let ns = row as NSString
        let all = NSRange(location: 0, length: ns.length)
        func first(_ r: NSRegularExpression, _ group: Int = 1) -> String? {
            guard let m = r.firstMatch(in: row, range: all), m.range(at: group).location != NSNotFound else { return nil }
            return ns.substring(with: m.range(at: group))
        }
        func duration(_ r: NSRegularExpression) -> Int? {
            guard let m = r.firstMatch(in: row, range: all),
                  let h = Int(ns.substring(with: m.range(at: 1))), let mi = Int(ns.substring(with: m.range(at: 2)))
            else { return nil }
            return (h * 60 + mi) * 60
        }

        guard let dayText = first(Self.dayRegex), let dayNumber = Int(dayText),
              let date = DayKey.calendar.date(from: DateComponents(year: year, month: month, day: dayNumber)),
              DayKey.calendar.component(.month, from: date) == month
        else { return }
        let day = DayKey.key(date)

        let status = first(Self.statusTypeRegex).map(DayStatus.init(hrmasterType:)) ?? .other
        let label = first(Self.statusLabelRegex)
            .map { TimeloggerParser.decodeEntities($0).trimmingCharacters(in: .whitespacesAndNewlines) } ?? ""
        let record = DayRecord(day: day, status: status, label: label,
                               reportedWorked: duration(Self.workedRegex), reportedBreaks: duration(Self.unpaidRegex))
        result.days.append(record)

        // Cells: each marker starts a cell that runs until the next marker.
        let cells = Self.cellRegex.matches(in: row, range: all)
        var workIndex = 0, breakIndex = 0
        var daySessions: [WorkSession] = []
        for (i, cell) in cells.enumerated() {
            guard cell.range(at: 1).location != NSNotFound else { continue } // paid-break column
            let isWork = ns.substring(with: cell.range(at: 1)) == "WorkScheduleRepeater"
            let start = cell.range.location + cell.range.length
            let end = i + 1 < cells.count ? cells[i + 1].range.location : ns.length
            let segment = ns.substring(with: NSRange(location: start, length: end - start))
            if isWork { workIndex += 1 } else { breakIndex += 1 }

            guard let pair = Self.times(in: segment) else { continue }
            let (from, to) = pair
            guard let s = Self.date(day, from), var e = Self.date(day, to) else {
                result.warnings.append("\(day): could not read \(from)–\(to).")
                continue
            }
            if e <= s { e = DayKey.calendar.date(byAdding: .day, value: 1, to: e)! }  // past midnight
            daySessions.append(WorkSession(id: nil, day: day, start: s, end: e, kind: isWork ? .work : .pause,
                                           label: isWork ? "Munkaidő \(workIndex)" : "Szünet \(breakIndex)"))
        }
        result.sessions += daySessions

        // Cross-check against the totals the HR system shows.
        let work = daySessions.filter { $0.kind == .work }.map { Interval(start: $0.start, end: $0.end) }
        let pauses = daySessions.filter { $0.kind == .pause }.map { Interval(start: $0.start, end: $0.end) }
        if !work.isEmpty, let reported = record.reportedWorked {
            let net = Int(Intervals.total(Intervals.subtract(work, pauses)))
            if abs(net - reported) > 60 {
                result.warnings.append("\(day): sessions add up to \(DurationFormat.string(net)), HRmaster says \(DurationFormat.string(reported)).")
            }
        }
    }

    /// The From and To values of a cell, either as rendered text or as an input's value (edit mode).
    private static func times(in segment: String) -> (String, String)? {
        let ns = segment as NSString
        let range = NSRange(location: 0, length: ns.length)
        var values = pickerRegex.matches(in: segment, range: range)
            .map { ns.substring(with: $0.range(at: 1)).trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
        if values.count < 2 {
            values = inputValueRegex.matches(in: segment, range: range).map { ns.substring(with: $0.range(at: 1)) }
        }
        guard values.count >= 2 else { return nil }
        return (values[0], values[1])
    }

    private static func date(_ day: String, _ hm: String) -> Date? {
        let ns = hm as NSString
        guard let m = timeRegex.firstMatch(in: hm, range: NSRange(location: 0, length: ns.length)),
              let h = Int(ns.substring(with: m.range(at: 1))), let mi = Int(ns.substring(with: m.range(at: 2)))
        else { return nil }
        if h == 24 && mi == 0 {   // "24:00" = midnight at the end of the day
            return DayKey.at(day, hour: 0, minute: 0).map { DayKey.calendar.date(byAdding: .day, value: 1, to: $0)! }
        }
        return DayKey.at(day, hour: h, minute: mi)
    }

    public init(fallbackYear: Int, fallbackMonth: Int) {
        self.fallbackYear = fallbackYear
        self.fallbackMonth = fallbackMonth
    }
}
