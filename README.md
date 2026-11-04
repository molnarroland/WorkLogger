# WorkLogger

A macOS SwiftUI app that turns your git history, Google Calendar meetings and the monthly report of your
work-time logger into reviewed timesheets, stored in a local SQLite file and exported to Tempo.

## Run it

Requires macOS 26+ (Liquid Glass) and Xcode 26+ (no third-party dependencies; uses the system SQLite, Charts and Network frameworks).

Open `WorkLogger.xcodeproj` in Xcode and press ⌘R (⌘U runs the tests). Every view has a `#Preview` backed by
sample data (`Support/PreviewData.swift`, throwaway database). Command line alternatives:

```bash
./scripts/make-app.sh  # builds build/WorkLogger.app (recommended: calendar permission sticks to the app)
swift run              # quick start from Terminal; or open Package.swift in Xcode → Run
```

Data lives in `~/Library/Application Support/WorkLogger/worklogs.sqlite`. Tokens are in the Keychain.

## Workflow

1. **Repositories** – choose your workspace folder; every git repo below it (4 levels deep) is listed.
   Enter your commit email(s). Optionally give a repo a default issue (`TAO-17`) or a project (`TAO`).
   Projects get a catch-all issue, because Tempo can only log to issues.
2. **Work sessions** – paste the monthly HTML (page source or copied text) of your work-time logger, choose
   Parse, check the preview, Import. Multiple sessions per day are fine; break rows (lunch, ebéd, szünet…)
   and the gaps between sessions are not working time.
3. **Calendar** – connect Google Calendar (see below) and add rules such as `standup → INT-12`.
4. **Timesheet** – Tempo-style month grid: each day shows logged vs. expected hours (green complete, orange partly,
   red nothing, purple leave). Click a day for its entries; shift-click a second day to mark the stretch as a period.
5. **Periods** – book a stretch of days to one issue for a larger project: commit time and the rest of each workday
   go to that issue (meetings keep theirs, leave is never filled). Rebuild drafts after changes.
6. **Review** – pick a range and choose **Collect**. Edit start, duration, issue or description inline.
   Edited entries lock automatically, so **Rebuild drafts** (after changing settings or sessions) keeps them.
7. **Totals** – daily / weekly / monthly hours per issue and per source.
8. **Export** (toolbar share icon) – copy or save a Markdown checklist that Claude with Claude in Chrome follows to
   fill your Tempo timesheet, push through the Tempo API, or save CSV / JSON.

## How times are calculated

Per day:

* **Working time** = imported work sessions − breaks (or the default workday when nothing was imported).
* **Meetings** keep their real slot and length (rounded to the granularity). Declined, all-day,
  out-of-office and focus-time events are ignored; you can untick any meeting.
* **Commits** get an estimate: `0.6 × free working minutes since your previous commit + 0.4 × complexity`,
  where complexity = `10 + 12·log2(1 + changed lines/10) + 4·files`, adjusted by the kind of change
  (refactor ×1.3, fix ×1.15, typo/bump/format ×0.5), clamped to min/max. Lockfiles, `vendor/`, `dist/`… are excluded.
* Commits are grouped per issue (or per repo when no key is found) into one entry with their subjects as description.
* **Daily total** – the time worked according to the imported sessions (HRmaster's “Ledolgozott órák”), rounded
  up to 15 minutes by default (7h 22m → 7h 30m, 12h → 12h); days without sessions use the hours per day (8h). Or keep
  raw estimates. Scaling uses the largest-remainder method on the rounding unit, so each day adds up exactly.
  Meetings and locked/manual entries are subtracted first; commit entries are placed into the remaining free slots.

Issue key resolution for commits: key in message → key in branch name (`feature/tao-123-…`) → repo default
issue → project catch-all issue. Unresolved entries are highlighted and never exported.

## HRmaster import

Copy the HTML of the Munkaidő table (Datacenter → Calendar, one month per paste, or several months at once) into the
Work sessions page. The parser reads the page through its `data-e2e-testing` attributes:

| What | Where it comes from |
| --- | --- |
| Month | period header, e.g. “2026. március” (falls back to the selected range's month) |
| Day type | `Type_CalendarDayStatus_*`: Workday, RestDay, PublicHoliday, LabourCode (leave) |
| Day label | e.g. “Munkanap (Home Office)”, “Szabadság”, “Pihenőnap” |
| Work sessions | Munkaidő 1–n columns (From/To) |
| Breaks | Szünet 1–n columns, subtracted from the sessions |
| Check totals | “Ledolgozott órák”; the preview warns when the sessions don't add up to it |

The HR day type decides the daily target, so transferred Saturday workdays count as workdays and weekday
holidays or leave never get filled with commit time. With an absence issue set (Settings → Absences), leave days
are logged with the hours HRmaster reports; label rules can send e.g. “Betegszabadság” to a different issue.
Leave days get only that one entry (meetings are not logged); commits made on a leave day are logged on the closest
workday (the earlier one on a tie), even when that day is just outside the selected range.
Commits on a weekend or other rest day without imported work sessions move the same way: Saturday → Friday,
Sunday → Monday. A rest day with work sessions keeps its commits.
The day's target is HRmaster's “Ledolgozott órák”, rounded up (Settings → Round worked time up to).

Run `swift test` to check the parser against fixture rows built from the real page markup.

## Google Calendar

No Google Cloud project or client ID is needed. The app reads calendars through macOS:

1. Calendar page → **Sign in with Google…** opens System Settings → Internet Accounts.
   Choose Add Account… → Google, sign in on Google's page, keep **Calendars** on.
2. **Allow calendar access** (one-time macOS prompt).
3. Tick the calendars to read (work calendar, shared team calendars…). New accounts may take a minute to sync.

Recurring meetings are expanded; all-day, cancelled and declined events are skipped. Access is read-only.

If you can't add the Google account to macOS (e.g. a managed Mac), the Advanced section still supports the
Google Calendar API with your own “Desktop app” OAuth client.

## Tempo

* **Push to Tempo** uses Tempo REST API v4 (`POST /4/worklogs`). It needs the numeric Jira issue id and your
  account id, which the app looks up through the Jira Cloud REST API (cached in SQLite). Settings needs the Jira site,
  email, Jira API token and a Tempo API token (Tempo → Settings → API Integration). One request per entry.
* **CSV** uses Tempo's worklog-import columns `Issue Key, Date Started, Time Spent, Comment`
  (times in UTC with a `Z` suffix). That importer belongs to Tempo's Time Tracker app; for Tempo Timesheets on
  Jira Cloud, the API push is the supported route.
* **JSON** contains the API v4 bodies (with issue keys) if you prefer to script the upload.

## Notes

* The work-time logger parser is format-agnostic. If your report uses a different date layout or labels,
  `Import/TimeloggerParser.swift` is the one file to adjust.
* The app is not sandboxed, because it runs `git` on your repositories.
