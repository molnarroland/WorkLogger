import Foundation

public struct DayInput: Sendable {
    public var day: String
    public var sessions: [WorkSession]
    public var commits: [StoredCommit]
    public var meetings: [Meeting]
    /// Entries that survive a rebuild (locked, reviewed, exported, manual).
    public var kept: [Worklog]
    /// Day status and totals from the HR system, when imported.
    public var info: DayRecord? = nil

    public init(
        day: String, sessions: [WorkSession], commits: [StoredCommit], meetings: [Meeting], kept: [Worklog],
        info: DayRecord? = nil
    ) {
        self.day = day
        self.sessions = sessions
        self.commits = commits
        self.meetings = meetings
        self.kept = kept
        self.info = info
    }
}

/// Turns one day's commits, meetings and work sessions into draft worklogs.
public struct WorklogBuilder: Sendable {
    public var settings: AppSettings
    public var repos: [Int64: Repo]
    public var projects: [String: ProjectMapping]

    public var knownPrefixes: Set<String> {
        var s = Set(projects.keys)
        for r in repos.values {
            if let p = r.projectKey { s.insert(p) }
            if let k = r.issueKey, let p = JiraKey.prefix(of: k) { s.insert(p) }
        }
        return s
    }

    // MARK: Issue resolution

    /// Commit message → branch name → repository issue → repository project's catch-all issue.
    public func issue(for c: StoredCommit) -> String? {
        let known = knownPrefixes
        if let k = JiraKey.find(in: c.subject + "\n" + c.body, known: known) { return k }
        if let k = JiraKey.find(in: c.ref.uppercased(), known: known) { return k }
        guard let repo = repos[c.repoId] else { return nil }
        if let k = repo.issueKey { return k }
        if let p = repo.projectKey, let d = projects[p]?.defaultIssue { return d }
        return nil
    }

    /// Title/description key → keyword rules → default meeting issue.
    public func issue(for m: Meeting) -> String? {
        if let k = JiraKey.find(in: m.title + "\n" + m.details, known: knownPrefixes) { return k }
        let title = m.title.lowercased()
        if let rule = settings.meetingRules.first(where: { !$0.contains.isEmpty && title.contains($0.contains.lowercased()) }) {
            return JiraKey.normalize(rule.issueKey)
        }
        return JiraKey.normalize(settings.defaultMeetingIssue)
    }

    public func isSkipped(_ m: Meeting) -> Bool {
        if m.skipped { return true }
        let title = m.title.lowercased()
        return settings.skipMeetingKeywords.contains { !$0.isEmpty && title.contains($0.lowercased()) }
    }

    // MARK: Day type & target

    /// The HR system's status wins over the calendar (Hungarian transferred Saturdays are workdays, a weekday holiday is not).
    public func isWorkday(_ day: String, info: DayRecord?) -> Bool {
        if let info, info.status != .other { return info.status == .workday }
        return !DayKey.isWeekend(day)
    }

    /// How long the day should add up to, or nil to keep the raw estimates.
    public func dailyTarget(day: String, info: DayRecord?, windows: [Interval], fromSessions: Bool) -> Int? {
        let fixed = Int(settings.fixedDailyHours * 3600)
        let worked = info?.reportedWorked.flatMap { $0 > 0 ? $0 : nil } ?? Int(Intervals.total(windows))
        let workday = isWorkday(day, info: info)
        let absence = info?.status.isAbsence ?? false

        guard settings.roundingMode != .estimate else { return nil }
        // Imported work sessions decide the day, rounded up (7h 22m → 7h 30m); the hours per day are only a fallback.
        if fromSessions {
            let step = max(1, settings.dayRoundUpMinutes) * 60
            return Int((Double(worked) / Double(step)).rounded(.up)) * step
        }
        if workday { return fixed }
        return settings.includeWeekends && !absence ? fixed : nil
    }

    /// Where a day's commits are logged: the day itself, or the closest workday (the earlier one on a tie) for
    /// leave (Szabadság…) and for rest days without work sessions, so Saturday goes to Friday and Sunday to Monday.
    /// Leave days themselves only get the absence entry.
    public func commitDay(for day: String, info: (String) -> DayRecord?, worked: (String) -> Bool = { _ in false }) -> String {
        let leave = info(day)?.status == .leave
        let idleRestDay = !isWorkday(day, info: info(day)) && !worked(day) && !settings.includeWeekends
        guard leave || idleRestDay else { return day }
        let date = DayKey.date(day)
        for offset in 1...31 {
            for sign in [-1, 1] {
                let d = DayKey.key(DayKey.calendar.date(byAdding: .day, value: sign * offset, to: date)!)
                if isWorkday(d, info: info(d)) { return d }
            }
        }
        return day
    }

    /// The project period a day belongs to; the latest added wins when periods overlap.
    public func period(for day: String) -> ProjectPeriod? {
        settings.periods.last { $0.contains(day) && JiraKey.normalize($0.issueKey) != nil }
    }

    /// Issue for a leave / holiday day: label rule ("Betegszabadság" → HR-3) → default absence issue.
    public func absenceIssue(for info: DayRecord) -> String? {
        let label = info.label.lowercased()
        if let rule = settings.absenceRules.first(where: { !$0.contains.isEmpty && label.contains($0.contains.lowercased()) }) {
            return JiraKey.normalize(rule.issueKey)
        }
        return JiraKey.normalize(settings.absenceIssueKey)
    }

    private func absenceWorklog(_ input: DayInput, usedRefs: Set<String>) -> Worklog? {
        guard let info = input.info, info.status.isAbsence, let issue = absenceIssue(for: info) else { return nil }
        let ref = "absence:\(input.day)"
        guard !usedRefs.contains(ref), !input.sessions.contains(where: { $0.kind == .work }) else { return nil }
        // Leave counts as a full day unless the HR system reports otherwise; a holiday only when it reports hours.
        let seconds = info.reportedWorked ?? (info.status == .leave ? Int(settings.fixedDailyHours * 3600) : 0)
        guard seconds > 0 else { return nil }
        let start = DayKey.at(input.day, hm: settings.defaultDayStart) ?? DayKey.date(input.day)
        return Worklog(
            id: 0, day: input.day, start: start, duration: seconds, issueKey: issue,
            description: info.label.isEmpty ? "Absence" : info.label, source: .absence, sourceRef: ref,
            repoId: nil, estimated: seconds, status: .draft, locked: false, tempoId: nil)
    }

    // MARK: Work windows

    /// Working time of the day: imported sessions minus breaks, or the default workday when nothing was imported.
    public func workWindows(_ input: DayInput) -> (windows: [Interval], fromSessions: Bool) {
        let pauses = input.sessions.pauseIntervals
        if !input.sessions.workIntervals.isEmpty { return (input.sessions.netWork, true) }
        guard let s = DayKey.at(input.day, hm: settings.defaultDayStart),
            let e = DayKey.at(input.day, hm: settings.defaultDayEnd), e > s
        else { return ([], false) }
        return (Intervals.subtract([Interval(start: s, end: e)], pauses), false)
    }

    // MARK: Build

    private struct Block {
        var key: String
        var issue: String?
        var repoId: Int64?
        var seconds: Int
        var subjects: [String]
        var hashes: [String]
        var period: ProjectPeriod? = nil
    }

    public func build(_ input: DayInput) -> [Worklog] {
        let hasWorkSessions = input.sessions.contains { $0.kind == .work }
        let usedRefs = Set(input.kept.flatMap(\.refs))
        let absence = absenceWorklog(input, usedRefs: usedRefs)
        // Leave is logged as one absence entry; commits were moved to the closest workday, meetings are not logged.
        if input.info?.status == .leave { return absence.map { [$0] } ?? [] }
        let period = period(for: input.day)
        let fillsPeriod = period != nil && (hasWorkSessions || isWorkday(input.day, info: input.info))
        guard !input.commits.isEmpty || !input.meetings.isEmpty || hasWorkSessions || absence != nil || fillsPeriod else {
            return []
        }

        let (windows, fromSessions) = workWindows(input)
        let gran = max(1, settings.granularityMinutes) * 60
        var out: [Worklog] = []
        if let absence { out.append(absence) }

        // 1. Meetings keep their real time slot.
        let meetings = input.meetings
            .filter { !isSkipped($0) && !usedRefs.contains($0.id) && $0.end > $0.start }
            .sorted { $0.start < $1.start }
        for m in meetings {
            let seconds = roundTo(m.duration, gran, minimum: gran)
            out.append(
                Worklog(
                    id: 0, day: input.day, start: m.start, duration: seconds, issueKey: self.issue(for: m),
                    description: m.title, source: .meeting, sourceRef: m.id, repoId: nil,
                    estimated: m.duration, status: .draft, locked: false, tempoId: nil))
        }
        let meetingIntervals =
            meetings.map { Interval(start: $0.start, end: $0.end) }
            + (absence.map { [Interval(start: $0.start, end: $0.end)] } ?? [])
        let keptIntervals = input.kept.map { Interval(start: $0.start, end: $0.end) }
        let free = Intervals.subtract(windows, meetingIntervals + keptIntervals)

        // 2. Estimate commits and group them per issue (or per repository when no issue is known).
        let commits = input.commits.filter { !usedRefs.contains($0.hash) }.sorted { $0.date < $1.date }
        let estimator = CommitEstimator(settings: settings)
        var blocks: [Block] = []
        var previous: Date? = windows.first?.start
        for c in commits {
            var gap: TimeInterval?
            // Commits moved here from a leave day have no gap on this day; they are estimated by size only.
            if let p = previous, c.date > p, DayKey.key(c.date) == input.day {
                gap = Intervals.overlap(free, with: Interval(start: p, end: c.date))
            }
            let seconds = estimator.estimateSeconds(c, freeGap: gap)
            previous = max(previous ?? c.date, c.date)

            let resolved = self.issue(for: c)
            let key = resolved ?? "repo:\(c.repoId)"
            let subject = JiraKey.stripLeadingKey(c.subject)
            if let i = blocks.firstIndex(where: { $0.key == key }) {
                blocks[i].seconds += seconds
                if !blocks[i].subjects.contains(subject) { blocks[i].subjects.append(subject) }
                blocks[i].hashes.append(c.hash)
            } else {
                blocks.append(
                    Block(
                        key: key, issue: resolved, repoId: c.repoId, seconds: seconds,
                        subjects: [subject], hashes: [c.hash]))
            }
        }

        // 3. Daily target.
        let target = dailyTarget(day: input.day, info: input.info, windows: windows, fromSessions: fromSessions)
        let fixedTotal = out.totalDuration + input.kept.totalDuration

        // Inside a project period all commit time goes to the period's issue, and a workday is filled with it.
        if let period, let issue = JiraKey.normalize(period.issueKey) {
            let fillRef = "period:\(period.id.uuidString):\(input.day)"
            let hashes = blocks.flatMap(\.hashes)
            if hashes.isEmpty && (!fillsPeriod || usedRefs.contains(fillRef)) {
                blocks = []
            } else {
                let seconds =
                    hashes.isEmpty
                    ? max(0, Int(settings.fixedDailyHours * 3600) - fixedTotal)
                    : blocks.reduce(0) { $0 + $1.seconds }
                blocks = [
                    Block(
                        key: issue, issue: issue, repoId: blocks.first?.repoId, seconds: seconds,
                        subjects: blocks.flatMap(\.subjects), hashes: hashes + [fillRef], period: period)
                ]
            }
        }
        let gitTarget = target.map { max(0, roundTo($0, gran) - fixedTotal) }
        let allocations = Self.allocate(blocks.map(\.seconds), target: gitTarget, granularity: gran)

        // 4. Place commit blocks into the free time, in the order the work happened.
        var cursor =
            free.first?.start ?? commits.first?.date ?? DayKey.at(input.day, hm: settings.defaultDayStart)
            ?? DayKey.date(input.day)
        for (i, b) in blocks.enumerated() where b.period == nil || allocations[i] > 0 {
            let (start, next) = Self.place(duration: allocations[i], from: cursor, in: free)
            cursor = next
            out.append(
                Worklog(
                    id: 0, day: input.day, start: start, duration: allocations[i], issueKey: b.issue,
                    description: describe(b), source: b.period == nil ? .git : .period,
                    sourceRef: b.hashes.joined(separator: ","),
                    repoId: b.repoId, estimated: b.seconds, status: .draft, locked: false, tempoId: nil))
        }
        return out.sorted { $0.start < $1.start }
    }

    private func describe(_ b: Block) -> String {
        var text = b.subjects.joined(separator: "; ")
        if let p = b.period {
            text = text.isEmpty ? p.title : p.title + ": " + text
        } else if b.issue == nil, let id = b.repoId, let repo = repos[id] {
            text = "[\(repo.name)] " + text
        }
        return text.count > 255 ? String(text.prefix(252)) + "…" : text
    }

    private func roundTo(_ seconds: Int, _ g: Int, minimum: Int = 0) -> Int {
        max(minimum, Int((Double(seconds) / Double(g)).rounded()) * g)
    }

    // MARK: Allocation & placement

    /// Splits `target` across blocks proportionally to their estimates, in whole granularity units
    /// (largest-remainder method, so the day sums exactly to the target).
    public static func allocate(_ weights: [Int], target: Int?, granularity g: Int) -> [Int] {
        guard !weights.isEmpty else { return [] }
        guard let target else {
            return weights.map { max(g, Int((Double($0) / Double(g)).rounded()) * g) }
        }
        let units = target / g
        guard units > 0 else { return weights.map { _ in 0 } }
        let total = Double(max(1, weights.reduce(0, +)))
        let exact = weights.map { Double($0) / total * Double(units) }
        var base = exact.map { Int($0.rounded(.down)) }
        var remaining = units - base.reduce(0, +)
        let order = exact.indices.sorted { (exact[$0] - Double(base[$0])) > (exact[$1] - Double(base[$1])) }
        for i in order where remaining > 0 {
            base[i] += 1
            remaining -= 1
        }
        // Every block gets at least one unit when there is room, taken from the largest.
        if units >= weights.count {
            for i in base.indices where base[i] == 0 {
                if let donor = base.indices.max(by: { base[$0] < base[$1] }), base[donor] > 1 {
                    base[donor] -= 1
                    base[i] = 1
                }
            }
        }
        return base.map { $0 * g }
    }

    /// Returns the start of a block and where the next block may begin, skipping meetings and breaks.
    public static func place(duration: Int, from cursor: Date, in free: [Interval]) -> (Date, Date) {
        var remaining = TimeInterval(duration)
        var start: Date?
        var t = cursor
        for slot in free where slot.end > t {
            let s = max(slot.start, t)
            if start == nil { start = s }
            let available = slot.end.timeIntervalSince(s)
            if remaining <= available { return (start!, s.addingTimeInterval(remaining)) }
            remaining -= available
            t = slot.end
        }
        let s = start ?? t
        return (s, t.addingTimeInterval(remaining))  // overflows past the last window
    }

    public init(settings: AppSettings, repos: [Int64: Repo], projects: [String: ProjectMapping]) {
        self.settings = settings
        self.repos = repos
        self.projects = projects
    }
}
