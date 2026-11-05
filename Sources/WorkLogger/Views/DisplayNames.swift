import SwiftUI
import WorkLoggerCore

// Translated names for the model enums. The models keep plain English strings (they are also used in exports and
// logs); everything the interface shows goes through these tables so the String Catalog picks them up.

extension Pane {
    var title: LocalizedStringResource {
        switch self {
        case .repositories: "Repositories"
        case .calendar: "Meetings"
        case .sessions: "Work sessions"
        case .periods: "Periods"
        case .chats: "Chats"
        case .timesheet: "Timesheet"
        case .review: "Review"
        case .byTicket: "By ticket"
        case .reports: "Totals"
        }
    }
}

extension Step {
    var title: LocalizedStringResource {
        switch self {
        case .setup: "1 · Set up"
        case .gather: "2 · Bring in the month"
        case .fill: "3 · Fill the timesheet"
        case .finish: "4 · Log in Tempo"
        }
    }
}

extension Module {
    var localizedTitle: LocalizedStringResource {
        switch self {
        case .git: "Git repositories"
        case .calendar: "Calendar meetings"
        case .workTimes: "Work times"
        case .chats: "Chats"
        }
    }
    var localizedSummary: LocalizedStringResource {
        switch self {
        case .git: "Turns your commits into worklogs. Leave off if you rarely code."
        case .calendar: "Logs meetings from any calendar synced to macOS: Google, Outlook / Exchange, iCloud, CalDAV."
        case .workTimes: "Clock-in times, leave and holidays, read from HRmaster after you sign in once."
        case .chats: "Google Chat, read in a built-in browser after you sign in once."
        }
    }
}

extension WorklogSource {
    var localizedLabel: LocalizedStringResource {
        switch self {
        case .git: "Coding"
        case .meeting: "Meetings"
        case .absence: "Absence"
        case .manual: "Manual"
        case .period: "Periods"
        case .chat: "Chats"
        }
    }
}

extension WorklogStatus {
    var localizedLabel: LocalizedStringResource {
        switch self {
        case .draft: "Draft"
        case .reviewed: "Reviewed"
        case .exported: "In Tempo"
        }
    }
}

extension RoundingMode {
    var localizedLabel: LocalizedStringResource {
        switch self {
        case .estimate: "Keep commit estimates"
        case .fixedHours, .sessions: "Scale each day to its worked time"
        }
    }
}

extension QuickRange {
    var localizedName: LocalizedStringResource {
        switch self {
        case .today: "Today"
        case .thisWeek: "This week"
        case .lastWeek: "Last week"
        case .thisMonth: "This month"
        case .lastMonth: "Last month"
        }
    }
}

extension Period {
    var localizedName: LocalizedStringResource {
        switch self {
        case .day: "Daily"
        case .week: "Weekly"
        case .month: "Monthly"
        }
    }
}
