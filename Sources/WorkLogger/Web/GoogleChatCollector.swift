import Foundation

/// Collects your Google Chat messages for a range inside the signed-in Google Chat window: the same Chat searches
/// the browser extension runs ("my messages", "mentions of me"), a week at a time.
@MainActor
struct GoogleChatCollector {
    let session: WebSession
    let email: String

    /// One message as the page script reports it.
    struct RawMessage: Decodable {
        var id: String
        var gid: String?
        var conv: String?
        var ts: Double
        var oneOnOne: Bool
        var sender: String?
        var me: Bool
        var text: String
        var extra: String?
    }

    static let searches: [(label: String, params: (String) -> [(String, String)])] = [
        ("my messages", { email in [("cfrom", email)] }),
        ("mentions of me", { _ in [("chasmention", "true")] }),
    ]

    func collect(from: Date, to: Date, progress: @escaping (String) -> Void) async throws -> ChatExport {
        guard await session.checkSignedIn() else { throw WebSessionError.notSignedIn(session.source.title) }
        session.open(URL(string: "https://chat.google.com/")!)
        try await session.waitUntilLoaded()
        let account = session.page.url?.path().firstMatch(of: /\/u\/(\d+)/).map { String($0.1) } ?? "0"

        var found: [String: RawMessage] = [:]
        let days = Self.days(from: from, to: to)
        for (i, day) in days.enumerated() {
            let start = day.start.timeIntervalSince1970 * 1000, end = day.end.timeIntervalSince1970 * 1000
            let title = "\(day.start.formatted(Date.FormatStyle(timeZone: DayKey.timeZone).weekday(.abbreviated).day().month(.abbreviated))) (\(i + 1) of \(days.count))"
            var today: Set<String> = []
            for search in Self.searches {
                // e.g. "Google Chat · Tue 3 Nov (2 of 30) · my messages · at 14:32 · 12 today · 140 in range"
                func show(scanned: Int, at ts: Double?) {
                    var line = "Google Chat · \(title) · \(search.label)"
                    if let ts { line += " · at \(DayKey.hm(Date(timeIntervalSince1970: ts / 1000)))" }
                    line += " · \(today.count + scanned) today · \(found.count + scanned) in range…"
                    progress(line)
                }
                show(scanned: 0, at: nil)
                session.open(Self.searchURL(account: account, params: search.params(email), day: day))
                try await session.waitUntilLoaded()
                let raw = try await session.run(Self.collectScript, progress: { json in
                    struct P: Decodable { var n: Int; var ts: Double? }
                    if let p = try? JSONDecoder().decode(P.self, from: Data(json.utf8)) { show(scanned: p.n, at: p.ts) }
                })
                guard let json = raw as? String,
                      let list = try? JSONDecoder().decode([RawMessage].self, from: Data(json.utf8))
                else { throw WebSessionError.script("Google Chat's page changed; the message list could not be read.") }
                for m in list where m.gid != nil && m.ts >= start && m.ts < end {
                    found[m.id] = m
                    today.insert(m.id)
                }
            }
        }
        return Self.export(Array(found.values), email: email)
    }

    /// One search per day, so progress can follow the day being read.
    nonisolated static func days(from: Date, to: Date) -> [(start: Date, end: Date)] {
        var out: [(Date, Date)] = []
        var start = DayKey.startOfDay(from)
        let end = DayKey.dayAfter(to)
        while start < end {
            let next = DayKey.dayAfter(start)
            out.append((start, next))
            start = next
        }
        return out
    }

    /// e.g. /u/0/app/search?q=&cfrom=me@x&cdatestart=2026-10-1&cdateend=2026-10-1&cdateoption=custom&cmembership=1
    nonisolated static func searchURL(account: String, params: [(String, String)], day: (start: Date, end: Date)) -> URL {
        let c = DayKey.calendar.dateComponents([.year, .month, .day], from: day.start)
        let urlDay = "\(c.year!)-\(c.month!)-\(c.day!)"
        var u = URLComponents(string: "https://chat.google.com/u/\(account)/app/search")!
        u.queryItems = ([("q", "")] + params + [("cdatestart", urlDay), ("cdateend", urlDay),
                         ("cdateoption", "custom"), ("cmembership", "1")]).map { URLQueryItem(name: $0.0, value: $0.1) }
        return u.url!
    }

    /// Groups messages into conversations, oldest first; my messages get my email as sender.
    nonisolated static func export(_ messages: [RawMessage], email: String) -> ChatExport {
        let sorted = messages.sorted { $0.ts < $1.ts }
        var order: [String] = []
        var groups: [String: [RawMessage]] = [:]
        for m in sorted {
            guard let gid = m.gid else { continue }
            if groups[gid] == nil { order.append(gid) }
            groups[gid, default: []].append(m)
        }
        return ChatExport(source: "google-chat", conversations: order.map { gid in
            let list = groups[gid]!
            return ChatExport.Conversation(
                id: gid, type: String(gid.split(separator: "/").first ?? ""), name: list[0].conv,
                messages: list.map { .init(time: Date(timeIntervalSince1970: $0.ts / 1000), sender: $0.me ? email : $0.sender,
                                           me: $0.me, text: $0.text, extra: $0.extra) })
        })
    }

    /// The extension's DOM reading (content.js, verified 2026-11), run as an async function body in the search
    /// results page. Returns the messages as a JSON string. If Chat changes its markup, fix these hooks first.
    static let collectScript = #"""
    const MSG = 'div[jsname="Ne3sFf"][data-id]';
    const TS = '[data-absolute-timestamp]';
    const CONV_NAME = '[data-name]';
    const MINE = '[data-is-viewer-message-creator]';
    const TEXT = '[jsname="bgckF"]';
    const RECEIPTS = '[jsname="QcZEyb"]';
    const CONV_REF = /\b(dm|space)\/([A-Za-z0-9_-]+)/;
    const SKIP = `${TEXT}, ${TS}, ${RECEIPTS}, [data-name], [role="list"], [role="button"], button, [role="tooltip"]`;
    const sleep = ms => new Promise(r => setTimeout(r, ms));
    const all = () => [...document.querySelectorAll(MSG)];
    function scroller() {
      for (let e = document.querySelector(MSG); e && e !== document.body; e = e.parentElement)
        if (/auto|scroll/.test(getComputedStyle(e).overflowY) && e.scrollHeight > e.clientHeight) return e;
    }
    async function waitFor(fn, ms) {
      for (const end = Date.now() + ms; Date.now() < end; await sleep(300)) if (fn()) return true;
      return false;
    }
    function extraText(m) {
      const parts = [];
      const w = document.createTreeWalker(m, NodeFilter.SHOW_TEXT);
      for (let n; (n = w.nextNode());) {
        const t = n.textContent.trim(), skip = n.parentElement.closest(SKIP);
        if (t && !(skip && m.contains(skip))) parts.push(t);
      }
      return [...new Set(parts)];
    }
    function parse(m) {
      const ref = m.getAttribute('jsdata')?.match(CONV_REF);
      const text = m.querySelector(TEXT)?.innerText.trim() || '';
      const parts = extraText(m);
      const prefix = parts.findIndex(t => /^[^:]{1,80}:$/.test(t));
      const sender = prefix >= 0 ? parts.splice(prefix, 1)[0].slice(0, -1) : null;
      const extra = parts.join(' | ');
      return {
        id: m.dataset.id,
        gid: ref ? `${ref[1]}/${ref[2]}` : null,
        conv: m.querySelector(CONV_NAME)?.getAttribute('data-name') || null,
        ts: +m.querySelector(TS)?.getAttribute('data-absolute-timestamp') || 0,
        oneOnOne: m.dataset.oneOnOneDm === 'true',
        sender,
        me: m.querySelector(MINE)?.getAttribute('data-is-viewer-message-creator') === 'true',
        text,
        extra: extra || null,
      };
    }
    // Results only load when you scroll through the last screen (jumping to the bottom does nothing).
    const got = new Map();
    let lastTs = null;
    window.__wlProgress = JSON.stringify({ n: 0 });
    // Progress for the status bar: messages read so far and the time of the last one on screen.
    const grab = () => {
      all().forEach(m => { const p = parse(m); got.set(m.dataset.id, p); if (p.ts) lastTs = p.ts; });
      window.__wlProgress = JSON.stringify({ n: got.size, ts: lastTs });
    };
    // An empty day shows Chat's "no results" text; don't wait the full 20 s for messages that never come.
    const empty = () => /Nincs találat|No results|Nincsenek találatok/i.test(document.body.innerText);
    await waitFor(() => all().length || empty(), 20000);
    if (all().length) {
      for (let stale = 0, i = 0; stale < 3 && i < 300; i++) {
        grab();
        const n = all().length, sc = scroller();
        if (!sc) break;
        sc.scrollTop = Math.max(0, sc.scrollHeight - sc.clientHeight - 800);
        await sleep(300);
        while (sc.scrollTop + sc.clientHeight < sc.scrollHeight - 2) { sc.scrollTop += 120; await sleep(120); }
        await sleep(2500);
        stale = all().length === n ? stale + 1 : 0;
      }
      grab();
    }
    return JSON.stringify([...got.values()].filter(m => m.ts));
    """#
}
