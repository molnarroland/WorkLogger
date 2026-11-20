package web

// hrmasterScript runs as an async function body with `year` and `month` (1–12): it steps the month selector to
// the wanted month and returns the calendar panel's HTML. Hooks are HRmaster's own test ids (data-e2e-testing).
const hrmasterScript = "const sleep = ms => new Promise(r => setTimeout(r, ms));\n" +
	"async function waitFor(fn, ms) {\n" +
	"  for (const end = Date.now() + ms; Date.now() < end; await sleep(250)) { const v = fn(); if (v) return v; }\n" +
	"  return null;\n" +
	"}\n" +
	"// After sign-in HRmaster lands on /Datacenter/ without the route; switching the hash keeps this script alive.\n" +
	"if (location.pathname.includes('/Account/Login')) return JSON.stringify({ error: 'no-panel' });\n" +
	"if (!location.hash.includes('employeeWorkingTime')) location.hash = '#/employeeWorkingTime/own';\n" +
	"const e2e = id => document.querySelector(`[data-e2e-testing=\"${id}\"]`);\n" +
	"const LABEL = 'Datacenter.Calendar.PeriodSelect', PREV = 'Datacenter.Calendar.ChangeMonth.Previous', NEXT = 'Datacenter.Calendar.ChangeMonth.Next';\n" +
	"const MONTHS = [\n" +
	"  ['január','február','március','április','május','június','július','augusztus','szeptember','október','november','december'],\n" +
	"  ['january','february','march','april','may','june','july','august','september','october','november','december'],\n" +
	"];\n" +
	"const panel = () => document.querySelector('ng-include[src*=\"calendar-right-panel-react\"]')\n" +
	"  || document.querySelector('#page-content > employee-calendar > div > div.bs4-row > div.bs4-col-12.bs4-col-md-8.bs4-col-xl-9 > ng-include');\n" +
	"function readLabel() {\n" +
	"  const el = e2e(LABEL);\n" +
	"  const text = el ? [...el.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent).join(' ').trim() : '';\n" +
	"  const m = text.match(/^(\\d{4})\\.?\\s+([^\\s\\d]+)$/);\n" +
	"  if (!m) return null;\n" +
	"  for (const list of MONTHS) { const i = list.indexOf(m[2].toLowerCase()); if (i >= 0) return { year: +m[1], month: i + 1, text }; }\n" +
	"  return null;\n" +
	"}\n" +
	"\n" +
	"// Signed in but no calendar: say what the page shows, so a slow page or changed markup isn't taken for a sign-out.\n" +
	"if (!(await waitFor(() => panel() && readLabel(), 30000))) {\n" +
	"  if (location.pathname.toLowerCase().includes('/account/login')) return JSON.stringify({ error: 'no-panel' });\n" +
	"  const el = e2e(LABEL);\n" +
	"  return JSON.stringify({ error: 'no-calendar', url: location.origin + location.pathname + location.hash,\n" +
	"    label: el ? el.textContent.trim().slice(0, 60) : '', panel: !!panel() });\n" +
	"}\n" +
	"let label = readLabel();\n" +
	"for (let step = 0; step < 36; step++) {\n" +
	"  const diff = (year * 12 + month) - (label.year * 12 + label.month);\n" +
	"  if (diff === 0) break;\n" +
	"  const arrow = e2e(diff > 0 ? NEXT : PREV);\n" +
	"  if (!arrow) return JSON.stringify({ error: 'no-arrows', label: label.text });\n" +
	"  const before = label.text;\n" +
	"  arrow.click();\n" +
	"  label = await waitFor(() => { const l = readLabel(); return l && l.text !== before ? l : null; }, 10000);\n" +
	"  if (!label) return JSON.stringify({ error: 'stuck', label: before });\n" +
	"}\n" +
	"if (label.year !== year || label.month !== month) return JSON.stringify({ error: 'unreachable' });\n" +
	"await sleep(2000);   // the table reloads its rows after the month changes\n" +
	"return JSON.stringify({ html: panel().outerHTML, label: label.text });"

// chatScript is the browser extension's DOM reading, run as an async function body in the search results page.
// Returns the messages as a JSON string. If Chat changes its markup, fix these hooks first.
const chatScript = "const MSG = 'div[jsname=\"Ne3sFf\"][data-id]';\n" +
	"const TS = '[data-absolute-timestamp]';\n" +
	"const CONV_NAME = '[data-name]';\n" +
	"const MINE = '[data-is-viewer-message-creator]';\n" +
	"const TEXT = '[jsname=\"bgckF\"]';\n" +
	"const RECEIPTS = '[jsname=\"QcZEyb\"]';\n" +
	"const CONV_REF = /\\b(dm|space)\\/([A-Za-z0-9_-]+)/;\n" +
	"const SKIP = `${TEXT}, ${TS}, ${RECEIPTS}, [data-name], [role=\"list\"], [role=\"button\"], button, [role=\"tooltip\"]`;\n" +
	"const sleep = ms => new Promise(r => setTimeout(r, ms));\n" +
	"const all = () => [...document.querySelectorAll(MSG)];\n" +
	"function scroller() {\n" +
	"  for (let e = document.querySelector(MSG); e && e !== document.body; e = e.parentElement)\n" +
	"    if (/auto|scroll/.test(getComputedStyle(e).overflowY) && e.scrollHeight > e.clientHeight) return e;\n" +
	"}\n" +
	"async function waitFor(fn, ms) {\n" +
	"  for (const end = Date.now() + ms; Date.now() < end; await sleep(300)) if (fn()) return true;\n" +
	"  return false;\n" +
	"}\n" +
	"function extraText(m) {\n" +
	"  const parts = [];\n" +
	"  const w = document.createTreeWalker(m, NodeFilter.SHOW_TEXT);\n" +
	"  for (let n; (n = w.nextNode());) {\n" +
	"    const t = n.textContent.trim(), skip = n.parentElement.closest(SKIP);\n" +
	"    if (t && !(skip && m.contains(skip))) parts.push(t);\n" +
	"  }\n" +
	"  return [...new Set(parts)];\n" +
	"}\n" +
	"function parse(m) {\n" +
	"  const ref = m.getAttribute('jsdata')?.match(CONV_REF);\n" +
	"  const text = m.querySelector(TEXT)?.innerText.trim() || '';\n" +
	"  const parts = extraText(m);\n" +
	"  const prefix = parts.findIndex(t => /^[^:]{1,80}:$/.test(t));\n" +
	"  const sender = prefix >= 0 ? parts.splice(prefix, 1)[0].slice(0, -1) : null;\n" +
	"  const extra = parts.join(' | ');\n" +
	"  return {\n" +
	"    id: m.dataset.id,\n" +
	"    gid: ref ? `${ref[1]}/${ref[2]}` : null,\n" +
	"    conv: m.querySelector(CONV_NAME)?.getAttribute('data-name') || null,\n" +
	"    ts: +m.querySelector(TS)?.getAttribute('data-absolute-timestamp') || 0,\n" +
	"    oneOnOne: m.dataset.oneOnOneDm === 'true',\n" +
	"    sender,\n" +
	"    me: m.querySelector(MINE)?.getAttribute('data-is-viewer-message-creator') === 'true',\n" +
	"    text,\n" +
	"    extra: extra || null,\n" +
	"  };\n" +
	"}\n" +
	"// Results only load when you scroll through the last screen (jumping to the bottom does nothing).\n" +
	"const got = new Map();\n" +
	"let lastTs = null;\n" +
	"window.__wlProgress = JSON.stringify({ n: 0 });\n" +
	"// Progress for the status bar: messages read so far and the time of the last one on screen.\n" +
	"const grab = () => {\n" +
	"  all().forEach(m => { const p = parse(m); got.set(m.dataset.id, p); if (p.ts) lastTs = p.ts; });\n" +
	"  window.__wlProgress = JSON.stringify({ n: got.size, ts: lastTs });\n" +
	"};\n" +
	"// An empty day shows Chat's \"no results\" text; don't wait the full 20 s for messages that never come.\n" +
	"const empty = () => /Nincs találat|No results|Nincsenek találatok/i.test(document.body.innerText);\n" +
	"await waitFor(() => all().length || empty(), 20000);\n" +
	"if (all().length) {\n" +
	"  for (let stale = 0, i = 0; stale < 3 && i < 300; i++) {\n" +
	"    grab();\n" +
	"    const n = all().length, sc = scroller();\n" +
	"    if (!sc) break;\n" +
	"    sc.scrollTop = Math.max(0, sc.scrollHeight - sc.clientHeight - 800);\n" +
	"    await sleep(300);\n" +
	"    while (sc.scrollTop + sc.clientHeight < sc.scrollHeight - 2) { sc.scrollTop += 120; await sleep(120); }\n" +
	"    await sleep(1500);\n" +
	"    stale = all().length === n ? stale + 1 : 0;\n" +
	"  }\n" +
	"  grab();\n" +
	"}\n" +
	"return JSON.stringify([...got.values()].filter(m => m.ts));"

// chatRPCScript runs Chat's own search request ("SBNmJb", what the search page sends) for my messages in
// [startMs, endMs), page by page, with the page's sign-in. Read from a HAR of chat.google.com, 2026-11; the
// request is undocumented, so if Chat changes it this is where to look. Returns {messages} or {error}.
const chatRPCScript = `
const W = window.WIZ_global_data || {};
const at = W.SNlM0e, me = W.qDCSke;
if (!at || !me) return JSON.stringify({ error: 'no sign-in token or user id on the page' });
const url = (account === '0' ? '' : '/u/' + account) + '/_/DynamiteWebUi/data/batchexecute?' + new URLSearchParams({
  rpcids: 'SBNmJb', 'source-path': '/app/search', bl: W.cfb2h || '', 'f.sid': W.FdrFJe || '', rt: 'c' });
const session = crypto.randomUUID().toUpperCase();
const out = [];
let token = null;
for (let page = 0; page < 200; page++) {
  // filter: [spaces, [[sender]], [start µs, end µs], , session, , 0, membership]
  const filter = [[], [[me]], [startMs * 1000, endMs * 1000], null, token ? null : session, null, 0, [[[[[[1]]]]], [[[1]]]]];
  const req = [null, token, null, '', null, session, filter, token ? Date.now() : null, [2], [340], 0, null, 1];
  const body = new URLSearchParams({ 'f.req': JSON.stringify([[['SBNmJb', JSON.stringify(req), null, token ? 'generic' : '2']]]), at });
  const r = await fetch(url, { method: 'POST', credentials: 'include', body,
    headers: { 'content-type': 'application/x-www-form-urlencoded;charset=UTF-8', 'x-same-domain': '1' } });
  if (!r.ok) return JSON.stringify({ error: 'HTTP ' + r.status });
  // ")]}'" then chunks of "<length>\n<json line>"; the answer is the "wrb.fr" row of our rpc id.
  let res;
  for (const line of (await r.text()).split('\n')) {
    if (!line.startsWith('[[')) continue;
    for (const row of JSON.parse(line)) if (row[0] === 'wrb.fr' && row[1] === 'SBNmJb') res = row;
  }
  if (!res || !res[2]) return JSON.stringify({ error: 'unexpected answer' + (res ? ' ' + JSON.stringify(res.slice(3)) : '') });
  const p = JSON.parse(res[2]);
  // An unnamed group chat is called by its members, as Chat shows it.
  const names = new Map((p[4] || []).map(g => [g?.[0]?.[0], g?.[2] || (g?.[17] || []).map(u => u?.[1]).filter(Boolean).join(', ') || null]));
  let older = true;
  for (const [m] of p[9] || []) {
    const ts = m?.[16], gid = m?.[2]?.[0];
    if (!ts || !gid) continue;
    if (ts >= startMs) older = false;
    out.push({ id: m[17]?.[0] || m[11], gid, conv: names.get(gid) || null, ts, me: true, text: m[5] || '' });
  }
  window.__wlProgress = JSON.stringify({ n: out.length, ts: out.length ? out[out.length - 1].ts : null });
  token = p[0];
  // Newest first: stop at the last page, or once a whole page is before the range.
  if (!token || older) break;
  await new Promise(r => setTimeout(r, 300)); // go easy between two requests
}
return JSON.stringify({ messages: out });
`
