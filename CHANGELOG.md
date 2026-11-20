# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/).

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

[0.1.1]: https://github.com/molnarroland/WorkLogger/releases/tag/0.1.1
[0.1.0]: https://github.com/molnarroland/WorkLogger/releases/tag/0.1.0
