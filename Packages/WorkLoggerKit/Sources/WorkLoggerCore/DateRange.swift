import Foundation

public enum QuickRange: String, CaseIterable {
    case today = "Today", thisWeek = "This week", lastWeek = "Last week", thisMonth = "This month", lastMonth = "Last month"
}

/// The selected days, inclusive. `from` and `to` may be in either order (the date pickers allow it).
public struct DateRange: Equatable {
    public var from: Date
    public var to: Date

    public var fromDay: String { DayKey.key(min(from, to)) }
    public var toDay: String { DayKey.key(max(from, to)) }
    /// Start of the first day.
    public var start: Date { DayKey.startOfDay(min(from, to)) }
    /// Start of the day after the last one (exclusive end).
    public var end: Date { DayKey.dayAfter(max(from, to)) }
    public var days: [String] { DayKey.days(from: from, to: to) }

    public init(from: Date, to: Date) {
        self.from = from
        self.to = to
    }

    /// The range covering a calendar interval; `to` is the last second inside it.
    public init(_ interval: DateInterval) {
        self.init(from: interval.start, to: interval.end.addingTimeInterval(-1))
    }

    public static func month(containing date: Date) -> DateRange? {
        DayKey.calendar.dateInterval(of: .month, for: date).map(DateRange.init)
    }

    public static func quick(_ r: QuickRange, now: Date = Date()) -> DateRange? {
        let cal = DayKey.calendar
        let interval: DateInterval?
        switch r {
        case .today: interval = cal.dateInterval(of: .day, for: now)
        case .thisWeek: interval = cal.dateInterval(of: .weekOfYear, for: now)
        case .lastWeek:
            interval = cal.date(byAdding: .day, value: -7, to: now).flatMap { cal.dateInterval(of: .weekOfYear, for: $0) }
        case .thisMonth: interval = cal.dateInterval(of: .month, for: now)
        case .lastMonth:
            interval = cal.date(byAdding: .month, value: -1, to: now).flatMap { cal.dateInterval(of: .month, for: $0) }
        }
        return interval.map(DateRange.init)
    }

    /// The whole month `delta` months away from the month the range starts in.
    public func shiftedByMonths(_ delta: Int) -> DateRange? {
        DayKey.calendar.date(byAdding: .month, value: delta, to: start).flatMap(DateRange.month(containing:))
    }
}
