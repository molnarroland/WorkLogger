# Product

<!-- impeccable:product-schema 1 -->

## Platform

macos

Native macOS app (macOS 26+, Swift 6, SwiftUI). This is binding: follow the macOS Human Interface Guidelines, Liquid Glass and system controls. Impeccable's platform list has no macOS value, so this is recorded as `macos` rather than reusing `ios`.

## Users

Colleagues at the user's company (Flexinform) who share the same setup: HRmaster for work times, Jira with Tempo for worklogs, git for code, macOS calendars and Google Chat. Most are developers who log time to Jira tickets at the end of the month. Tech leads and managers, whose days are mostly meetings and chats, use it too.

Their job: turn a month that has already happened into a correct timesheet in Tempo or Jira that matches HR's records, without rebuilding it from `git log`, the calendar and chat history by hand.

The README also names contractors with several Jira sites and teams on other HR systems. The product supports them (several Jira accounts, the "Another tool" provider), but they are not the primary audience.

## Product Purpose

WorkLogger reads what the user actually did (commits, meetings, chats, HRmaster work sessions), estimates the time for each piece of work, fits it into the hours actually worked, and gives the user a timesheet to review. The user decides what goes in. WorkLogger does the typing: by hand from the By ticket list, through the Tempo or Jira API, or through a Markdown checklist that Claude in Chrome follows.

It succeeds when month-end booking is quick, every day adds up to HR's "Ledolgozott órák", and nothing is booked twice.

## Positioning

Reconstruction after the fact, not a timer. It derives worklogs from evidence the user already produced and fits them to HR's recorded hours, with an exact per-day sum (largest-remainder scaling). Edits the user makes are locked and survive rebuilds. A start/stop tracker cannot offer this.

## Operating Context

- Mostly used at month end, in one sitting, working through the month day by day and ticket by ticket.
- The workflow: Repositories → Work sessions (HRmaster fetch) → Calendar rules → Timesheet month grid → Periods → Review (Collect, edit, Rebuild drafts) → Chats → Log in Tempo (By ticket, Totals) → Export.
- Signed-in WebKit windows for HRmaster and Google Chat. The macOS Keychain holds the tokens and is unlocked with Touch ID at launch.
- A menu bar clock shows today's logged time. Keyboard shortcuts: ⌘1–⌘9 for pages, ⌘R Collect, ⇧⌘R Rebuild drafts, ⌘[ / ⌘] to change month, ⌘L Log in Tempo.

## Capabilities and Constraints

- Sidebar pages: Timesheet, Review, By ticket, Totals, Work sessions, Meetings, Chats, Repositories, plus Settings (General, Jira accounts, Absences, Chats). Read-back of existing Tempo/Jira worklogs.
- The current redesign (`feature/redesign`) is a **visual refresh with the same flows**: the pages, the sidebar structure and the workflow order stay. Only the look and feel changes.
- Timesheet day states and their meaning: complete, partly logged, nothing logged, leave (currently green / orange / red / purple). The four states are product truth. The colors are the current implementation, not a commitment.
- Unresolved entries (no issue key) are highlighted and never exported. Edited entries lock.
- No third-party dependencies (system SQLite, Charts, Network). Not sandboxed, because it runs `git`.
- Localized in English and Hungarian (String Catalogs). Hungarian strings run longer and layouts must fit them. HRmaster terms (Munkaidő, Ledolgozott órák, Szabadság) appear as data.
- Terminology: worklog, entry, draft, issue/ticket, period, work session, stretch (of chatting), Collect, Rebuild drafts, read-back, "In Tempo".

## Brand Commitments

- Name: WorkLogger. App icon assets: `Sources/WorkLogger/AppIcon.icon` (clock, grid, sheet).
- Voice (from the README): plain, direct and mildly wry ("Your month already happened. WorkLogger writes the timesheet for it."). The user stays in control: "You decide what goes in. WorkLogger only does the typing."

## Evidence on Hand

- Screenshots of every main page from sample data: `docs/screenshots/` (timesheet, review, by-ticket, totals, work-sessions, chats, locked). They are regenerated with `WorkLogger -WLScreenshots <folder>` (`Support/ScreenshotTour.swift`).
- `#Preview`s on every view, backed by `Support/PreviewData.swift`.
- No testimonials, usage numbers or adoption figures exist. Do not invent any.

## Product Principles

1. **The user decides; the app types.** Every entry can be reviewed and edited, and edits are never overwritten.
2. **Agree with HR.** Each day's total matches the recorded hours exactly. Leave and holidays are never filled with work.
3. **Never book twice.** What is already in Tempo or Jira is always visible next to the drafts.
4. **Local and private.** Data stays in a local SQLite file, tokens stay locked until Touch ID, and message contents are never logged.
5. **Native first.** The app behaves like a good Mac app: system controls, shortcuts, menu bar and localization.
