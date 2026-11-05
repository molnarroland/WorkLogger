import Foundation
import WorkLoggerCore

/// Reads the monthly work-time calendar ("Munkaidő") from the signed-in HRmaster window: opens the page, steps the
/// month selector to each month of the range and hands the calendar panel's HTML to `HRmasterParser`.
@MainActor
public struct HRmasterCollector {
    public let session: any PageDriver
    public let base: URL  // the company's HRmaster, e.g. https://flexinform.hrmaster.hu

    public struct Month: Hashable {
        public var year: Int
        public var month: Int
        public init(year: Int, month: Int) {
            self.year = year
            self.month = month
        }
    }

    private struct Result: Decodable {
        var html: String?
        var label: String?
        var error: String?
    }

    /// The work-time page of a company's HRmaster.
    public static func workingTimeURL(_ base: URL) -> URL {
        URL(
            string: base.absoluteString.trimmingCharacters(in: CharacterSet(charactersIn: "/"))
                + "/Datacenter/#/employeeWorkingTime/own")!
    }

    /// "flexinform.hrmaster.hu", "https://flexinform.hrmaster.hu/Datacenter/…" → https://flexinform.hrmaster.hu
    public nonisolated static func normalize(_ text: String) -> URL? {
        var t = text.trimmingCharacters(in: .whitespaces)
        if t.isEmpty { return nil }
        if !t.contains("://") { t = "https://" + t }
        guard let c = URLComponents(string: t), let host = c.host, host.contains(".") else { return nil }
        return URL(string: "\(c.scheme ?? "https")://\(host)")
    }

    public nonisolated static func months(from: Date, to: Date) -> [Month] {
        let cal = DayKey.calendar
        var out: [Month] = []
        var d = cal.date(from: cal.dateComponents([.year, .month], from: from))!
        while d <= to {
            let c = cal.dateComponents([.year, .month], from: d)
            out.append(Month(year: c.year!, month: c.month!))
            d = cal.date(byAdding: .month, value: 1, to: d)!
        }
        return out
    }

    /// The calendar panel's HTML per month.
    public func collect(_ months: [Month], progress: (String) -> Void) async throws -> [(Month, String)] {
        session.open(Self.workingTimeURL(base))
        try await session.waitUntilLoaded()
        var out: [(Month, String)] = []
        for (i, m) in months.enumerated() {
            let name = DayKey.calendar.date(from: DateComponents(year: m.year, month: m.month, day: 1))!
                .formatted(Date.FormatStyle(timeZone: DayKey.timeZone).month(.wide).year())
            progress(String(localized: "HRmaster: reading \(name) (\(i + 1) of \(months.count))…", bundle: .module))
            guard let json = try await session.run(Self.script, arguments: ["year": m.year, "month": m.month]) as? String,
                let r = try? JSONDecoder().decode(Result.self, from: Data(json.utf8))
            else { throw WebSessionError.script(String(localized: "HRmaster: the page could not be read.", bundle: .module)) }
            if let error = r.error {
                Log.web.error("HRmaster \(m.year)-\(m.month): \(error, privacy: .public)")
                if error == "no-panel" { throw WebSessionError.notSignedIn("HRmaster") }
                throw WebSessionError.script("HRmaster: \(error)")
            }
            Log.web.info("HRmaster \(m.year)-\(m.month): \(r.label ?? "-", privacy: .public), \(r.html?.count ?? 0) characters")
            out.append((m, r.html ?? ""))
        }
        return out
    }

    /// Runs as an async function body with `year` and `month` (1–12). Reads the period label ("2026. november"),
    /// clicks the previous/next month arrows until it shows the wanted month, then returns the calendar panel's HTML.
    /// Hooks are HRmaster's own test ids (data-e2e-testing), checked on the live page 2026-11.
    public static let script = #"""
        const sleep = ms => new Promise(r => setTimeout(r, ms));
        async function waitFor(fn, ms) {
          for (const end = Date.now() + ms; Date.now() < end; await sleep(250)) { const v = fn(); if (v) return v; }
          return null;
        }
        // After sign-in HRmaster lands on /Datacenter/ without the route; switching the hash keeps this script alive.
        if (location.pathname.includes('/Account/Login')) return JSON.stringify({ error: 'no-panel' });
        if (!location.hash.includes('employeeWorkingTime')) location.hash = '#/employeeWorkingTime/own';
        const e2e = id => document.querySelector(`[data-e2e-testing="${id}"]`);
        const LABEL = 'Datacenter.Calendar.PeriodSelect', PREV = 'Datacenter.Calendar.ChangeMonth.Previous', NEXT = 'Datacenter.Calendar.ChangeMonth.Next';
        const MONTHS = [
          ['január','február','március','április','május','június','július','augusztus','szeptember','október','november','december'],
          ['january','february','march','april','may','june','july','august','september','october','november','december'],
        ];
        const panel = () => document.querySelector('ng-include[src*="calendar-right-panel-react"]')
          || document.querySelector('#page-content > employee-calendar > div > div.bs4-row > div.bs4-col-12.bs4-col-md-8.bs4-col-xl-9 > ng-include');
        function readLabel() {
          const el = e2e(LABEL);
          const text = el ? [...el.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent).join(' ').trim() : '';
          const m = text.match(/^(\d{4})\.?\s+([^\s\d]+)$/);
          if (!m) return null;
          for (const list of MONTHS) { const i = list.indexOf(m[2].toLowerCase()); if (i >= 0) return { year: +m[1], month: i + 1, text }; }
          return null;
        }

        if (!(await waitFor(() => panel() && readLabel(), 30000))) return JSON.stringify({ error: 'no-panel' });
        let label = readLabel();
        for (let step = 0; step < 36; step++) {
          const diff = (year * 12 + month) - (label.year * 12 + label.month);
          if (diff === 0) break;
          const arrow = e2e(diff > 0 ? NEXT : PREV);
          if (!arrow) return JSON.stringify({ error: `the month arrows next to "${label.text}" were not found.` });
          const before = label.text;
          arrow.click();
          label = await waitFor(() => { const l = readLabel(); return l && l.text !== before ? l : null; }, 10000);
          if (!label) return JSON.stringify({ error: `the month arrow next to "${before}" did not change the month.` });
        }
        if (label.year !== year || label.month !== month) return JSON.stringify({ error: `could not reach ${year}-${month}.` });
        await sleep(2000);   // the table reloads its rows after the month changes
        return JSON.stringify({ html: panel().outerHTML, label: label.text });
        """#

    public init(session: any PageDriver, base: URL) {
        self.session = session
        self.base = base
    }
}
