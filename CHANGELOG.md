# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.3.1] - 2026-11-25

### Fixed

- **Memory kept growing** while the app ran: the signed-in pages of the Google and HRmaster profiles stayed open
  hidden. They now close once a job ends and when their window is closed with no job running; the next fetch signs
  in again from the saved sign-in.
- **Removing a profile** closes its window too.

## [0.3.0] - 2026-11-23

### Added

- **Days and Sources in the side panel**: a switcher next to the fold button. Days has Overview, Days, Week, the days
  left to do and the month's pages (Timesheet, Review, Log in Tempo). Sources lists each source with its count for the
  month, or "Set up" while it still needs something, then Periods and Plans, Finish setup and the modules.
- **A Setup tab on every source page**: Repositories (folders, commit emails, skipped folders), Meetings (calendars),
  Work sessions, Chats, Jira, Confluence and Sent mail (their account). The Activity tab says what is still missing,
  with a Set up button, and the setup checklist links straight to it.
- **A guided first run**: what a month looks like, what you use, each account in turn (with Skip for now), then the
  first Collect, which ends on the first day that needs work.
- **Next up on the Overview**: the first day that needs work and what it needs.
- **WORKLOGGER_PROFILE**: run on a separate data folder and keychain service, to try something on a fresh database.

### Changed

- **Collect** has its name in the header, not only an icon.
- **Days and Week** keep only their own ‹ Today › arrows; the month moves with the picker and ⌘[ / ⌘].
- **An entry without a ticket** shows the ticket field first in the details panel.
- **Log in Tempo** is one side panel item, with By ticket and Totals as its tabs.
- **Settings** has no month header or source bar, and lays out like the system's own: a centred column on macOS and
  Linux, the whole width on Windows. Calendars moved to Meetings → Setup.
- **Icons** are Phosphor; the open page's icon is filled.

### Removed

- **All pages** and pinning pages to the side panel: every page has a place in Days or Sources.

## [0.2.1] - 2026-11-22

### Changed

- **Unlocking asks who you are**: Touch ID or your login password on macOS, Windows Hello (face, fingerprint or PIN)
  on Windows. Cancelling leaves the app locked. Linux unlocks without asking, as before.

## [0.2.0] - 2026-11-22

### Added

- **A commit's contents**: a git entry's Commits button (in the inspector and on the Plans page) shows the commits it
  was made from, with their message, the files each changed and each file's changes (or the whole commit's).
- **Choose a ticket** for an entry without one: from the tickets planned for the day, used that day, the project
  periods', or used in this period or the months before it; or search Jira for issues created within one or two
  weeks of the day, each shown with the start of its description. In the inspector (Choose…) and on the Plans page,
  for one entry or all of a day's.
- The Plans page also lists days that have no plans but have entries without a ticket.
- **Send a week or a month to Tempo** at once: the Week page's send button and Export → Send reviewed entries send
  the period's reviewed entries, after a question that says how many go and which days are left out (days that
  still have drafts, and days that haven't come yet). What Tempo already has for those days is read back and left
  out, as for a single day.

### Fixed

- **Collect reads HRmaster's hours** that weren't final when last read: today's, and any day read before it ended
  (once, also the days read by an earlier version). Before, a day read in the middle of it kept its partial hours,
  and today was left out, so the hours had to be fetched by hand.

## [0.1.1] - 2026-11-20

### Fixed

- **Jira tokens with scopes** now work on every page straight after starting the app. Before, some pages didn't
  recognise the token until it was tested on the Profiles page: WorkLogger now checks once per site whether the
  token has to go through api.atlassian.com.

### Changed

- CI runs only formatting, lint and the tests, on Linux; the release builds are made locally with `task release`.

## [0.1.0] - 2026-11-20

The first public release.

### Added

- **Draft worklogs, built per day** from the sources you turn on:
  - Repositories: git commits, estimated from their size and timing.
  - Meetings: Google Calendar or an ICS feed, with their real length.
  - Chats: stretches of Google Chat conversation, logged to the conversation's ticket.
  - Work sessions: HRmaster sessions, which also fill the gaps between other entries on Collect.
  - Jira activity: tickets you created, changed or commented on, with suggested keys for untagged entries.
  - Confluence pages: pages and blog posts you wrote, edited or commented on, with a suggested time.
  - Sent mail: subjects and times read from the Sent folder over IMAP, booked to a ticket named in the subject.
  - Plans: Tempo Planner plans, offered as the ticket for matching entries.
- **Tickets** from commit messages and branch names, rules per repository, project, meeting and chat, and project
  periods that book or top up whole days.
- **Pages:** Overview, Days, Week, Timesheet, Review, By ticket and Totals, plus a page per source. Pin the pages
  you use to the side panel and open the rest from a launcher.
- **Review and edit:** select entries anywhere on the row, edit and move them in a resizable side panel, and mark
  them reviewed.
- **Send to Tempo** one day at a time from the Days page: only reviewed entries go, on the issue's Tempo account,
  and what Tempo already has for the day is left out and shown. Or export a Markdown checklist, CSV or JSON.
- **Privacy:** an encrypted database, tokens in the system keyring, and an app that starts locked. The activity
  console lists every request with secrets masked.
- **First run:** an onboarding guide that picks the sources, how worklogs reach Tempo, and the accounts each
  source needs.
- English and Hungarian; looks and behaves like a native app on macOS, Windows and Linux.
- Downloads, none of them code-signed:
  - macOS: the Apple silicon app in a DMG.
  - Windows: the 64-bit exe in a zip.
  - Linux: the 64-bit binary, compressed with UPX.

[0.3.1]: https://github.com/molnarroland/WorkLogger/releases/tag/0.3.1
[0.3.0]: https://github.com/molnarroland/WorkLogger/releases/tag/0.3.0
[0.2.1]: https://github.com/molnarroland/WorkLogger/releases/tag/0.2.1
[0.2.0]: https://github.com/molnarroland/WorkLogger/releases/tag/0.2.0
[0.1.1]: https://github.com/molnarroland/WorkLogger/releases/tag/0.1.1
[0.1.0]: https://github.com/molnarroland/WorkLogger/releases/tag/0.1.0
