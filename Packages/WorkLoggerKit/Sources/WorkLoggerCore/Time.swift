import Foundation

/// Day keys ("yyyy-MM-dd" in the local time zone) and calendar helpers. Weeks start on Monday (ISO).
public enum DayKey: Sendable {
    private struct Config {
        let calendar: Calendar
        let formatter: DateFormatter

        init(timeZone: TimeZone, locale: Locale) {
            var c = Calendar(identifier: .gregorian)
            c.firstWeekday = 2
            c.minimumDaysInFirstWeek = 4
            c.timeZone = timeZone
            c.locale = locale
            let f = DateFormatter()
            f.calendar = c
            f.locale = Locale(identifier: "en_US_POSIX")
            f.timeZone = timeZone
            f.dateFormat = "yyyy-MM-dd"
            calendar = c
            formatter = f
        }
    }

    private static let lock = NSLock()
    nonisolated(unsafe) private static var config = Config(timeZone: .current, locale: .current)

    /// Switches the time zone (and locale) every day key is computed in. The app keeps the system's;
    /// tests pin one so results don't depend on the machine they run on.
    public static func configure(timeZone: TimeZone, locale: Locale = .current) {
        let new = Config(timeZone: timeZone, locale: locale)
        lock.withLock { config = new }
    }

    public static var calendar: Calendar { lock.withLock { config.calendar } }
    public static var timeZone: TimeZone { calendar.timeZone }
    private static var formatter: DateFormatter { lock.withLock { config.formatter } }

    public static func key(_ d: Date) -> String { formatter.string(from: d) }
    public static func parse(_ k: String) -> Date? { formatter.date(from: k).map { calendar.startOfDay(for: $0) } }
    public static func date(_ k: String) -> Date { parse(k) ?? calendar.startOfDay(for: Date()) }
    public static func startOfDay(_ d: Date) -> Date { calendar.startOfDay(for: d) }
    public static func dayAfter(_ d: Date) -> Date { calendar.date(byAdding: .day, value: 1, to: startOfDay(d))! }

    public static func bounds(_ day: String) -> (Date, Date) {
        let s = date(day)
        return (s, dayAfter(s))
    }

    public static func days(from: Date, to: Date) -> [String] {
        var out: [String] = []
        var d = startOfDay(min(from, to))
        let end = startOfDay(max(from, to))
        while d <= end {
            out.append(key(d))
            d = calendar.date(byAdding: .day, value: 1, to: d)!
        }
        return out
    }

    public static func at(_ day: String, hour: Int, minute: Int) -> Date? {
        guard let d = parse(day), (0...23).contains(hour), (0...59).contains(minute) else { return nil }
        return calendar.date(bySettingHour: hour, minute: minute, second: 0, of: d)
    }

    public static func at(_ day: String, hm: String) -> Date? {
        let p = hm.split(separator: ":").compactMap { Int($0.trimmingCharacters(in: .whitespaces)) }
        guard p.count >= 2 else { return nil }
        return at(day, hour: p[0], minute: p[1])
    }

    public static func isWeekend(_ day: String) -> Bool { calendar.isDateInWeekend(date(day)) }

    public static func hm(_ d: Date) -> String {
        let c = calendar.dateComponents([.hour, .minute], from: d)
        return String(format: "%02d:%02d", c.hour ?? 0, c.minute ?? 0)
    }

    public static func title(_ day: String) -> String {
        date(day).formatted(.dateTime.weekday(.abbreviated).day().month(.abbreviated).year())
    }
}

public struct Interval: Hashable, Sendable {
    public var start: Date
    public var end: Date
    public var duration: TimeInterval { max(0, end.timeIntervalSince(start)) }

    public init(start: Date, end: Date) {
        self.start = start
        self.end = end
    }
}

public enum Intervals: Sendable {
    public static func normalize(_ list: [Interval]) -> [Interval] {
        var out: [Interval] = []
        for i in list.filter({ $0.end > $0.start }).sorted(by: { $0.start < $1.start }) {
            if let last = out.last, i.start <= last.end {
                out[out.count - 1].end = max(last.end, i.end)
            } else {
                out.append(i)
            }
        }
        return out
    }

    public static func subtract(_ base: [Interval], _ cuts: [Interval]) -> [Interval] {
        var result = normalize(base)
        for c in normalize(cuts) {
            result = result.flatMap { b -> [Interval] in
                if c.end <= b.start || c.start >= b.end { return [b] }
                var parts: [Interval] = []
                if c.start > b.start { parts.append(Interval(start: b.start, end: c.start)) }
                if c.end < b.end { parts.append(Interval(start: c.end, end: b.end)) }
                return parts
            }
        }
        return result
    }

    public static func total(_ list: [Interval]) -> TimeInterval {
        normalize(list).reduce(0) { $0 + $1.duration }
    }

    public static func overlap(_ list: [Interval], with w: Interval) -> TimeInterval {
        normalize(list).reduce(0) { acc, i in
            acc + max(0, min(i.end, w.end).timeIntervalSince(max(i.start, w.start)))
        }
    }
}

public enum DurationFormat: Sendable {
    public static func string(_ seconds: Int) -> String {
        let minutes = Int((Double(seconds) / 60).rounded())
        let h = minutes / 60
        let m = minutes % 60
        if h == 0 { return "\(m)m" }
        return m == 0 ? "\(h)h" : "\(h)h \(m)m"
    }

    public static func hours(_ seconds: Int) -> String {
        String(format: "%.2f h", Double(seconds) / 3600)
    }

    private static let unitRegex = try! NSRegularExpression(pattern: #"(\d+(?:\.\d+)?)\s*([hm])"#)

    /// Accepts "1h 30m", "1.5h", "90m", "1:30" or a bare number of minutes ("90").
    public static func parse(_ text: String) -> Int? {
        let t = text.lowercased().trimmingCharacters(in: .whitespaces).replacingOccurrences(of: ",", with: ".")
        guard !t.isEmpty else { return nil }
        if t.contains(":") {
            let p = t.split(separator: ":").map { Int($0.trimmingCharacters(in: .whitespaces)) }
            guard p.count == 2, let h = p[0], let m = p[1] else { return nil }
            return (h * 60 + m) * 60
        }
        if let minutes = Double(t) { return Int((minutes * 60).rounded()) }
        let ns = t as NSString
        var total = 0.0
        let matches = unitRegex.matches(in: t, range: NSRange(location: 0, length: ns.length))
        guard !matches.isEmpty else { return nil }
        for m in matches {
            let v = Double(ns.substring(with: m.range(at: 1))) ?? 0
            total += ns.substring(with: m.range(at: 2)) == "h" ? v * 3600 : v * 60
        }
        return Int(total.rounded())
    }
}
