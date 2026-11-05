import Foundation

/// Parses pasted HTML (or the text copied from the rendered page) of a monthly work-time report.
///
/// It is deliberately format-agnostic: the text is split into rows, a row containing a date sets the current day,
/// and every time range on a row ("08:02 – 12:15", or two separate time cells) becomes a session.
/// Rows mentioning a break keyword (lunch, ebéd, szünet…) become breaks. Multiple sessions per day are fine;
/// the gaps between them are treated as non-working time.
public struct TimeloggerParser {
    public var breakKeywords: [String]
    public var shortDateMonthFirst: Bool
    public var defaultYear: Int = DayKey.calendar.component(.year, from: Date())

    /// Detects the format: HRmaster's monthly table gets its dedicated parser, anything else the generic one.
    public func parseTimesheet(_ raw: String, fallbackMonth: Int) -> ParsedTimesheet {
        if HRmasterParser.matches(raw) {
            return HRmasterParser(fallbackYear: defaultYear, fallbackMonth: fallbackMonth).parse(raw)
        }
        return ParsedTimesheet(sessions: parse(raw), days: [], format: "Generic", warnings: [])
    }

    public func parse(_ raw: String) -> [WorkSession] {
        var currentDay: String?
        var contextYear = defaultYear
        var out: [WorkSession] = []

        for line in Self.lines(from: raw) {
            if let found = findDate(in: line, contextYear: contextYear) {
                currentDay = found.0
                contextYear = found.1
            }
            guard let day = currentDay else { continue }
            let lower = line.lowercased()
            let isPause = breakKeywords.contains { !$0.isEmpty && lower.contains($0.lowercased()) }

            for (s, e) in Self.findRanges(in: line) {
                guard let start = DayKey.at(day, hour: s.0, minute: s.1),
                      var end = DayKey.at(day, hour: e.0, minute: e.1) else { continue }
                if end <= start { end = DayKey.calendar.date(byAdding: .day, value: 1, to: end)! } // past midnight
                out.append(WorkSession(id: nil, day: day, start: start, end: end, kind: isPause ? .pause : .work,
                                       label: String(line.prefix(80))))
            }
        }

        var seen = Set<String>()
        return out.filter { seen.insert("\($0.day)|\($0.start)|\($0.end)|\($0.kind)").inserted }
            .sorted { $0.start < $1.start }
    }

    // MARK: HTML → rows

    public static func lines(from raw: String) -> [String] {
        var s = raw
        s = s.replacingOccurrences(of: #"(?is)<(script|style)[^>]*>.*?</\1>"#, with: " ", options: .regularExpression)
        s = s.replacingOccurrences(of: #"(?i)<br\s*/?>|</(tr|p|div|li|h[1-6]|table|section|article|ul|ol)>"#,
                                   with: "\n", options: .regularExpression)
        s = s.replacingOccurrences(of: #"(?i)</t[dh]>"#, with: "\t", options: .regularExpression)
        s = s.replacingOccurrences(of: #"<[^>]+>"#, with: " ", options: .regularExpression)
        s = decodeEntities(s)
        return s.components(separatedBy: .newlines)
            .map { $0.replacingOccurrences(of: #"[ \u{00a0}]+"#, with: " ", options: .regularExpression)
                .trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
    }

    public static func decodeEntities(_ s: String) -> String {
        var r = s
        let named: [(String, String)] = [("&nbsp;", " "), ("&lt;", "<"), ("&gt;", ">"), ("&quot;", "\""),
                                         ("&#39;", "'"), ("&apos;", "'"), ("&ndash;", "–"), ("&mdash;", "—"),
                                         ("&amp;", "&")]
        for (k, v) in named { r = r.replacingOccurrences(of: k, with: v) }

        let regex = try! NSRegularExpression(pattern: "&#(x?)([0-9a-fA-F]+);")
        let ns = r as NSString
        var result = ""
        var last = 0
        for m in regex.matches(in: r, range: NSRange(location: 0, length: ns.length)) {
            result += ns.substring(with: NSRange(location: last, length: m.range.location - last))
            let isHex = ns.substring(with: m.range(at: 1)) == "x"
            if let code = UInt32(ns.substring(with: m.range(at: 2)), radix: isHex ? 16 : 10),
               let scalar = Unicode.Scalar(code) {
                result.append(Character(scalar))
            } else {
                result += ns.substring(with: m.range)
            }
            last = m.range.location + m.range.length
        }
        result += ns.substring(from: last)
        return result
    }

    // MARK: Dates

    private static let isoDate = try! NSRegularExpression(
        pattern: #"(?<!\d)(20\d{2})\s*[.\-/]\s*(\d{1,2})\s*[.\-/]\s*(\d{1,2})(?!\d)"#)
    private static let dayFirstDate = try! NSRegularExpression(
        pattern: #"(?<!\d)(\d{1,2})\s*([./])\s*(\d{1,2})\s*[./]\s*(20\d{2})(?!\d)"#)
    private static let shortDate = try! NSRegularExpression(
        pattern: #"(?<![\d.:])(\d{1,2})\.\s?(\d{1,2})\.(?![\d:])"#)

    /// Returns ("yyyy-MM-dd", year). Supports 2026-11-03, 2026. 11. 03., 03.11.2026, 11/03/2026 and short 11.03.
    public func findDate(in line: String, contextYear: Int) -> (String, Int)? {
        let ns = line as NSString
        let range = NSRange(location: 0, length: ns.length)
        func int(_ m: NSTextCheckingResult, _ i: Int) -> Int { Int(ns.substring(with: m.range(at: i))) ?? 0 }

        if let m = Self.isoDate.firstMatch(in: line, range: range) {
            return Self.make(int(m, 1), int(m, 2), int(m, 3))
        }
        if let m = Self.dayFirstDate.firstMatch(in: line, range: range) {
            let a = int(m, 1), b = int(m, 3), y = int(m, 4)
            let usStyle = ns.substring(with: m.range(at: 2)) == "/"
            return usStyle ? Self.make(y, a, b) : Self.make(y, b, a)
        }
        if let m = Self.shortDate.firstMatch(in: line, range: range) {
            let a = int(m, 1), b = int(m, 2)
            return shortDateMonthFirst ? Self.make(contextYear, a, b) : Self.make(contextYear, b, a)
        }
        return nil
    }

    private static func make(_ y: Int, _ m: Int, _ d: Int) -> (String, Int)? {
        guard (1...12).contains(m), (1...31).contains(d) else { return nil }
        return (String(format: "%04d-%02d-%02d", y, m, d), y)
    }

    // MARK: Times

    private static let rangeRegex = try! NSRegularExpression(
        pattern: #"(?<![\d:])([01]?\d|2[0-3]):([0-5]\d)(?::[0-5]\d)?\s*(?:-|–|—|to|->|→|ig)\s*([01]?\d|2[0-3]):([0-5]\d)(?::[0-5]\d)?(?![\d:])"#)
    private static let timeRegex = try! NSRegularExpression(
        pattern: #"(?<![\d:])([01]?\d|2[0-3]):([0-5]\d)(?::[0-5]\d)?(?![\d:])"#)

    public typealias HM = (Int, Int)

    /// Explicit ranges first; otherwise consecutive standalone times are paired (start, end).
    /// A lone time on a row (e.g. a daily total) is ignored.
    public static func findRanges(in line: String) -> [(HM, HM)] {
        let ns = line as NSString
        let range = NSRange(location: 0, length: ns.length)
        func int(_ m: NSTextCheckingResult, _ i: Int) -> Int { Int(ns.substring(with: m.range(at: i))) ?? 0 }

        let explicit = rangeRegex.matches(in: line, range: range)
        if !explicit.isEmpty {
            return explicit.map { ((int($0, 1), int($0, 2)), (int($0, 3), int($0, 4))) }
        }
        let times = timeRegex.matches(in: line, range: range).map { (int($0, 1), int($0, 2)) }
        guard times.count >= 2 else { return [] }
        return stride(from: 0, to: times.count - 1, by: 2).map { (times[$0], times[$0 + 1]) }
    }

    public init(breakKeywords: [String], shortDateMonthFirst: Bool, defaultYear: Int = DayKey.calendar.component(.year, from: Date())) {
        self.breakKeywords = breakKeywords
        self.shortDateMonthFirst = shortDateMonthFirst
        self.defaultYear = defaultYear
    }
}
