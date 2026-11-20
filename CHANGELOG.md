# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and versions follow [Semantic Versioning](https://semver.org/).

## [0.1.0] - unreleased

The first public release.

### Added

- Draft worklogs built per day from git commits (estimated from size and timing), calendar meetings (Google
  Calendar or ICS feeds), Google Chat conversations and HRmaster work sessions.
- Tickets from commit messages and branch names, rules per repository, project, meeting and chat, and project
  periods that book whole days.
- Overview, Days, Week, Timesheet, Review, By ticket and Totals pages; edit and move entries and mark them reviewed.
- Send to Tempo or Jira over their APIs by hand, one day at a time from the Days page (only reviewed entries; what
  Tempo already has for the day is left out), or export as Markdown, CSV or JSON; what Tempo already has is read
  back and shown.
- Tokens in the system keyring; the app starts locked. An activity console lists every request with secrets masked.
- English and Hungarian; looks native on macOS, Windows and Linux.
- Packages for macOS (universal), Windows (zipped installer) and Linux (AppImage, deb, rpm), not code-signed.

[0.1.0]: https://github.com/molnarroland/WorkLogger/releases/tag/v0.1.0
