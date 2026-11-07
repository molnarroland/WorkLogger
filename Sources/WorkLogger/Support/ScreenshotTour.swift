#if DEBUG
    import AppKit
    import Foundation
    import WorkLoggerCore
    import WorkLoggerStore

    /// Debug builds only: launch with `-WLScreenshots <folder>` to open a month of sample data, show each page and save
    /// the main window as PNGs into the folder (the README's screenshots), then quit. The window draws itself, so no
    /// Screen Recording permission is needed.
    @MainActor
    enum ScreenshotTour {
        static var folder: String? { UserDefaults.standard.string(forKey: "WLScreenshots") }
        static var isActive: Bool { folder != nil }
        /// `-WLScreenshotSize 720x900` checks a smaller window (half-screen tiling); the README size is the default.
        static var size: NSSize {
            let parts = (UserDefaults.standard.string(forKey: "WLScreenshotSize") ?? "").split(separator: "x").compactMap {
                Double($0)
            }
            return parts.count == 2 ? NSSize(width: parts[0], height: parts[1]) : NSSize(width: 1360, height: 860)
        }

        static func run(_ state: AppState) async {
            guard let folder else { return }
            let dir = URL(fileURLWithPath: folder, isDirectory: true)
            try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
            try? await Task.sleep(for: .seconds(1))
            window?.setFrame(NSRect(origin: NSPoint(x: 80, y: 80), size: size), display: true)
            for (pane, name) in [
                (Pane.timesheet, "timesheet"), (.review, "review"), (.byTicket, "by-ticket"), (.reports, "totals"),
                (.sessions, "work-sessions"), (.chats, "chats"), (.periods, "periods"), (.calendar, "meetings"),
                (.repositories, "repositories"),
            ] {
                state.requestedPane = pane
                try? await Task.sleep(for: .seconds(1.5))
                save(name, in: dir)
            }
            state.locked = true
            try? await Task.sleep(for: .seconds(1))
            save("locked", in: dir)
            NSApp.terminate(nil)
        }

        private static var window: NSWindow? { NSApp.windows.first { $0.title == "WorkLogger" && $0.isVisible } }

        private static func save(_ name: String, in dir: URL) {
            guard let id = window?.windowNumber, let image = capture(CGWindowID(id)) else { return }
            let rep = NSBitmapImageRep(cgImage: image)
            try? rep.representation(using: .png, properties: [:])?.write(to: dir.appendingPathComponent("\(name).png"))
        }

        /// The window as the window server draws it, Liquid Glass included. `CGWindowListCreateImage` is hidden from
        /// the macOS 15+ SDK but still works for the app's own windows without Screen Recording permission.
        private static func capture(_ id: CGWindowID) -> CGImage? {
            typealias Create = @convention(c) (CGRect, UInt32, CGWindowID, UInt32) -> Unmanaged<CGImage>?
            guard let symbol = dlsym(UnsafeMutableRawPointer(bitPattern: -2), "CGWindowListCreateImage") else { return nil }
            let create = unsafeBitCast(symbol, to: Create.self)
            // .optionIncludingWindow, .boundsIgnoreFraming | .bestResolution
            return create(.null, 1 << 3, id, 1 << 0 | 1 << 3)?.takeRetainedValue()
        }
    }

    extension AppState {
        /// Last month, filled like a real one: work sessions, a leave day, standups, commit work on a few tickets,
        /// chats, an entry still without a ticket, and the first half already in Tempo.
        static var demo: AppState {
            let url = FileManager.default.temporaryDirectory.appendingPathComponent("WorkLogger-demo-\(UUID().uuidString).sqlite")
            let state = AppState(databaseURL: url)
            state.locked = false
            let db = state.db
            let lastMonth = DayKey.calendar.date(byAdding: .month, value: -1, to: Date())!
            let month = DateRange.month(containing: lastMonth)!
            let workdays = DateRange(from: month.from, to: month.to).days.filter { !DayKey.isWeekend($0) }
            let tickets = [
                ("TAO-123", "Fix invoice rounding; add tests"), ("TAO-131", "Refactor payment client"),
                ("TAO-140", "CSV export for accounting"), ("WEB-58", "Dashboard filters and saved views"),
            ]
            var sessions: [WorkSession] = []
            var days: [DayRecord] = []
            for (i, day) in workdays.enumerated() {
                func at(_ hm: String) -> Date { DayKey.at(day, hm: hm)! }
                func log(_ hm: String, _ minutes: Int, _ issue: String?, _ text: String, _ source: WorklogSource) {
                    let status: WorklogStatus = i < workdays.count / 2 ? .exported : .draft
                    _ = try? db.insertWorklog(
                        Worklog(
                            id: 0, day: day, start: at(hm), duration: minutes * 60, issueKey: issue, description: text,
                            source: source, sourceRef: "", repoId: nil, estimated: minutes * 60, status: status,
                            locked: false, tempoId: nil))
                }
                if i == 6 {  // a day of leave
                    days.append(
                        DayRecord(day: day, status: .leave, label: "Szabadság", reportedWorked: 8 * 3600, reportedBreaks: 0))
                    log("08:00", 480, "HR-1", "Szabadság", .absence)
                    continue
                }
                sessions += [
                    WorkSession(id: nil, day: day, start: at("08:30"), end: at("17:00"), kind: .work, label: "Munkaidő 1"),
                    WorkSession(id: nil, day: day, start: at("12:00"), end: at("12:30"), kind: .pause, label: "Ebéd"),
                ]
                days.append(
                    DayRecord(
                        day: day, status: .workday, label: "Munkanap (Home Office)", reportedWorked: 8 * 3600,
                        reportedBreaks: 1800))
                if i >= workdays.count - 2 { continue }  // the last days are still empty
                let a = tickets[i % tickets.count]
                let b = tickets[(i + 1) % tickets.count]
                log("08:30", 15, "INT-12", "Daily standup", .meeting)
                log("08:45", 195, a.0, a.1, .git)
                log("12:30", 30, "TAO-123", "TAO space: release questions", .chat)
                if i == workdays.count - 4 {
                    log("13:00", 240, nil, "Code review", .manual)  // still needs a ticket
                } else if i % 5 == 3 {
                    log("13:00", 60, "INT-7", "Sprint planning", .meeting)
                    log("14:00", 180, b.0, b.1, .git)
                } else {
                    log("13:00", 240, b.0, b.1, .git)
                }
            }
            try? db.importTimesheet(ParsedTimesheet(sessions: sessions, days: days, format: "HRmaster"))
            let iso = ISO8601DateFormatter()
            let chatDay = workdays[workdays.count - 3]
            _ = state.importChats(
                """
                {"source":"google-chat","conversations":[{"id":"space/TAO","type":"space","name":"TAO","messages":[
                  {"time":"\(iso.string(from: DayKey.at(chatDay, hm: "12:31")!))","sender":"me@x","me":true,"text":"Deploying the rounding fix to test"},
                  {"time":"\(iso.string(from: DayKey.at(chatDay, hm: "12:40")!))","sender":"bea@x","me":false,"text":"Thanks, checking now"},
                  {"time":"\(iso.string(from: DayKey.at(chatDay, hm: "12:52")!))","sender":"me@x","me":true,"text":"Release notes are in the ticket"}]}]}
                """)
            state.settings.welcomed = true
            state.setRange(from: month.from, to: month.to)
            state.reloadAll()
            return state
        }
    }
#endif
