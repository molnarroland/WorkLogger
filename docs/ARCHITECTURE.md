# WorkLogger architecture

WorkLogger is one Go process with a webview frontend, built with [Wails v3](https://v3alpha.wails.io) (pinned to
`v3.1.0`). The Go side collects your work (git commits, calendar meetings, Google Chat conversations, HRmaster work
sessions), turns each day into draft worklogs, and sends the reviewed ones to Tempo or Jira. The Angular frontend
shows what the backend computed and sends back what you change.

## Principles

1. **The backend computes, the frontend shows.** Day states, totals, to-dos, ticket groups and what may be edited are
   decided in Go (`internal/view`, `app/`). The frontend formats and lays them out; it doesn't re-derive them.
2. **A pure core with thin edges.** The domain logic (`internal/core`, `internal/view`) does no I/O and never reads the
   clock. Disk, network, git, web pages and time sit behind small interfaces, so every edge can be faked in tests.
3. **Rules live in Go.** What the UI hides (deleting an entry that is already in Tempo, an invalid setting) is also
   refused by the bound method.
4. **Few dependencies.** SQLite through `modernc.org/sqlite` (pure Go, no cgo), the OS keyring through
   `github.com/zalando/go-keyring`, and the UI on Angular, spartan-ng and Spartan Native. No ORM, no state library.

## Repository layout

```
main.go                 wiring: database, keyring, windows, the runtime guard, events to the main window
app/                    the Wails service: every method the frontend can call, by feature
internal/
  core/                 domain logic: models, settings, the worklog builder, commit estimates, messages
  view/                 what each screen shows, computed from core types and "today"
  store/                SQLite: schema migrations, queries, the draft rebuild
  git/                  repository discovery and history through the git command line
  calendar/             Google Calendar (OAuth loopback + PKCE) and ICS feeds
  tempo/                Jira Cloud, Confluence and Tempo REST clients, export formats, the connection checks
  imap/                 the Sent folder's subjects and times over IMAP (TLS), with no IMAP library
  web/                  HRmaster and Google Chat collectors and the scripts they run in those pages
  webview/              the site windows those collectors drive, the script bridge, the runtime guard
  platform/             real adapters: clock, keyring, process runner
  testsupport/          fakes and fixture builders shared by tests
frontend/
  src/app/              the Angular app (see Frontend)
  bindings/             TypeScript generated from the Go service (task bindings)
  libs/                 vendored Spartan Native packages and generated spartan-ng components
third_party/            the vendored Spartan Native Go host module
build/                  Wails packaging config per OS
site/                   the GitHub Pages site
```

## Backend

### Ports

`internal/core/ports.go` names everything the core needs from the outside world: `Clock`, `SecretStore`,
`HTTPDoer`, `CommandRunner` and `PageDriver` (a signed-in web page that can be opened and run scripts). `main.go`
passes the real adapters (`internal/platform`, `internal/webview`); tests pass fakes from `internal/testsupport`.

### Days and time

A day is a `"2006-01-02"` key in one time zone, handled by `core.Days{Loc}` (`Key`, `Parse`, `Range`, `Shift`,
`Bounds`). The app passes the system zone; tests pin Europe/Budapest, so daylight-saving days are covered.
`main.go` imports `time/tzdata` because Windows has no zoneinfo database.

### Building drafts

`core.Builder.Build` turns one day's commits, meetings, chats and work sessions into draft worklogs:

- **Absence** first: a leave day is one absence entry and nothing else; commits on leave days and idle rest days
  move to the closest workday (`CommitDay`).
- **Meetings** keep their real time slot. Their issue comes from a key in the title or details, then a keyword
  rule, then the default meeting issue.
- **Commits** are grouped by issue (from the commit message, the branch, the repository, then the repository's
  project default). Each commit's time is estimated (`EstimateSeconds`: `10 + 12·log2(1 + churn/10) +
  4·min(files, 20)` minutes, scaled by keywords in the subject, blended with the free time since the previous commit
  when that is known), then the day's target is shared out by largest remainder and placed in the free intervals.
- **Project periods** book whole days to one issue; **long chats** become drafts of their own.

The day's target comes from the settings' rounding mode: fixed hours, the estimate as it is, or HRmaster's work
sessions.

`store.Rebuild` reruns the builder for a range in one transaction: regenerable drafts are deleted, and entries that
survive (edited, reviewed, exported or manual ones) keep the commits and meetings they cover, so nothing is logged
twice.

### Store

One SQLite file at `os.UserConfigDir()/WorkLogger/worklogger.sqlite`, opened with WAL and foreign keys on. The schema
is a list of migrations (`internal/store/store.go`), applied in order and tracked in `PRAGMA user_version`; new
steps are appended, old ones never change. Settings are one JSON document in the `settings` table, merged with the
defaults on load and normalised (`core.Settings.Normalize`), so an older or hand-edited database can't put the UI
into a state it doesn't know.

### The service

`app.Service` is the one Wails service. Its exported methods are the frontend's API; they take and return plain
DTOs (`app/dto.go`: string day keys, integer seconds) and stay thin: validate, call core, store or a client, and
return what the screen shows. Settings are changed by intent (`AddPeriod`, `SetChatRule`, `UpdateProfile`, …) and
each edit is validated on a copy before it is saved.

Long jobs (collect, rebuild, fetching HRmaster, Google Chat or Google Calendar) go through `startJob`/`finish`
(`app/jobs.go`): one at a time, cancellable, reporting progress as `collect:progress` events and the result as
`collect:done`. Tempo's read-back runs separately (`RefreshTempo`) and is cached per day, so showing a month never
waits for the network.

What the user is told is a `core.Message`: a code with parameters and its English text. Errors the user can act on
are `core.UserError`s carrying a message; the frontend translates the code (`msg.<code>` in its dictionaries) and
falls back to the English text. `internal/core/messages.go` lists every code, and a test checks both dictionaries
have them.

Every outgoing request and git command is written to the activity console (`app/activity.go`) with credentials
masked; chat message text never is.

### Integrations

| Source | How |
| --- | --- |
| git | `git log` with a fixed format through `CommandRunner`; on Windows without a console window |
| Jira and Tempo | REST (Jira v3, Tempo v4), one client per Jira profile; tokens from the keyring |
| Confluence | REST (CQL search v1, page versions v2) with a linked Jira profile's token or its own; scoped tokens through api.atlassian.com |
| Email | IMAP over TLS (`internal/imap`): the Sent folder opened read-only, only Date and Subject headers read; the password in the keyring |
| Google Calendar | the Calendar API with your own OAuth client (loopback redirect + PKCE), or the calendar's export read through the signed-in page |
| ICS feeds | read-only URLs, parsed in `internal/calendar/ics.go` |
| HRmaster, Google Chat | the signed-in web pages, driven in app windows (below) |

### Signed-in web pages

HRmaster and Google Chat have no API WorkLogger can use, so each such profile gets an app window of its own
(`internal/webview/session.go`, window name `web-<profile id>`). You sign in there once; fetching drives the same
page, with the window hidden unless it needs you.

1. The window first opens the app's `web-session.html`, which tells Wails it is ready, then navigates to the site.
2. Collector scripts (`internal/web/scripts.go`) run in the page and post their results back as `wl:<id>:<json>`
   messages; the `Bridge` accepts each answer only from the window that ran the script.
3. The user agent ends in Safari's, which Google accepts for sign-in.
4. Wails has no cookie API: signed-in is decided by where the site's home page lands, and signing out uses the
   site's own logout.

These scripts depend on the pages' markup, so a change on Google's or HRmaster's side can break them.
`web.CollectHRmaster` and `web.CollectGoogleChat` see only a `PageDriver`, and their tests use a fake page.

## Frontend

### The `Api` seam

Components never call Wails. `src/app/api/api.ts` defines the `Api` interface and the DTO types; `WailsApi`
implements it with the generated bindings, and `FakeApi` with in-memory data for Storybook, tests and `npm start` in
a browser. `app.config.ts` picks one at startup; the fake and its data load in a separate chunk, only outside Wails.
`api/bindings-check.ts` fails the build when a hand-written DTO and the generated one have different fields.

The fake's sample month comes from the Go views (`task fixtures` writes `api/testing/sample.json`), so stories show
what the backend would compute.

### State and structure

`state/shell.store.ts` (`ShellStore`) holds the open month as the backend returned it, the settings, the job status
and the status line. Pages read signals from it and call its methods or the `Api`; a failed call ends in the status
bar (`ShellStore.attempt`).

| Folder | Holds |
| --- | --- |
| `shell/` | The window: sidebar, toolbar, status bar, shortcuts, the macOS menu, the export flow |
| `pages/` | One folder per page, lazy-loaded from `app.routes.ts` |
| `ui/` | Components with inputs and outputs only, no store or API of their own |
| `state/` | The store and other injectable state |
| `shared/` | Pure functions, pipes and constants |
| `i18n/` | `en.json`, `hu.json` and the `t` pipe; the language is a setting ("system" follows the OS) |

The look comes from Spartan Native, which dresses the window as the OS it runs on (or the one chosen in Settings →
General) and reads the OS accent, light or dark, contrast and text size through its Go service.

## Testing

- **Go:** unit tests next to the code; the store tests use a real SQLite database (in memory, or a file in a temp dir); HTTP clients are tested
  against `httptest` servers and the collectors against fake pages. `task test` runs them with `-race`.
- **Frontend:** Vitest specs for the store, pipes and pure functions; every component in `ui/`, `pages/` and
  `shell/` has a Storybook story (a spec checks this), and the stories' `play` functions are interaction tests,
  run with an accessibility check (`npm run test-storybook`). The shell stories cover the main flow end to end
  against the fake API.
- **CI** (`.github/workflows/ci.yml`) runs formatting, lint, both test suites, the Storybook tests, a build on
  macOS, Windows and Linux, and a check that the generated bindings are up to date.

## Trust boundaries

WorkLogger is one Go process with a webview frontend. It holds API tokens (Jira, Tempo, Google), reads your git
history, calendars, chats and HR records, and can send worklogs to Tempo or Jira. The boundaries below decide who can
make it do any of that.

### The app's own window is the only caller

The frontend (Angular, in the main window) calls the Go service's bound methods through the Wails runtime: a
`POST /wails/runtime` to the app's asset server. Events from Go reach it as JavaScript run in the window.

**Site windows are not trusted.** Google Chat, Google Calendar (without the API) and HRmaster are read by driving
their signed-in web pages in app windows of their own (`internal/webview/session.go`, window names `web-<profile id>`). Those
windows show pages from other origins, which could be malicious or compromised.

What we found in Wails v3.1.0 (`pkg/application`, checked 2026-11-13):

- **Runtime calls are not origin-checked on desktop.** `checkRuntimeRequest` is a no-op in desktop builds
  (`transport_http_notserver.go`); the Origin check exists only in server mode. A runtime call is a plain POST,
  which a page can send without a CORS preflight, so it runs even though the page can't read the answer.
  Whether a foreign page can reach the asset server depends on the platform: macOS WebKit refuses a fetch from
  `https://…` to `wails://localhost` ("Load failed", seen in a probe); WebView2 on Windows intercepts
  `http://wails.localhost/*` for every page in the webview.
- **Events go to every window.** `App.Event.Emit` runs `window._wails.dispatchWailsEvent(…)` in each window that has
  reported ready, and site windows report ready (our `web-session.html` does, so scripts can run there). A page that
  defines `window._wails.dispatchWailsEvent` would receive every event: collect progress, chat days, activity lines.
- **Raw messages are delivered from any page** to `Options.RawMessageHandler`, with the sending window.

What WorkLogger does about it:

| Boundary | Where | Rule |
| --- | --- | --- |
| Runtime requests | `internal/webview/guard.go` `GuardRuntime`, the asset server's outermost middleware | Every `/wails/…` request (runtime calls, the event stream, event payloads, the runtime script) is refused with 403 when it comes from a site window (by the `x-wails-window-name` Wails sets, which a page can't override), or when its Origin isn't the app's own (`wails://localhost`, `http://wails.localhost`, with a port in dev), or when it is cross-site without an Origin. Refusals are logged to stderr and the activity console. |
| Events | `main.go` `Shell.Emit` | The app's events are dispatched to the main window only (`WebviewWindow.DispatchWailsEvent`), never broadcast. Spartan Native's own appearance events (light/dark, accent) are still broadcast; they carry no user data. |
| Script results | `internal/webview/session.go` `Bridge` | A `wl:<id>:<result>` message is taken only from the window that ran that script, and the id is 96 random bits. |
| Secrets | `app/jira.go` | No bound method returns a token. The frontend learns only whether one is saved (`HasToken`, `HasGoogleClientSecret`) and can replace it; keyring names are made in Go and the kind is checked (`jira`, `tempo`). |
| Backend rules | `app/worklogs.go`, `app/settings_edit.go`, `app/profiles_edit.go` | Rules are enforced in Go, not in menus: entries sent to Tempo can't be changed or deleted, settings edits are validated and applied to a copy. |

Checked in the running app (macOS, a throwaway build that opened a hidden site window and ran a probe in it):
the main window's runtime calls pass (`window-1`, no Origin on same-origin requests, all answered 200); from the
app's start page in a site window, `POST /wails/runtime` is refused with 403; from `https://example.com` in a site
window, a fetch to `wails://localhost/wails/runtime` is blocked by WebKit before it is sent. `internal/webview/guard_test.go`
covers the rules, including the Windows origin.

### Data at rest and on the network

- **Local data** lives in one SQLite file, `os.UserConfigDir()/WorkLogger/worklogger.sqlite` (directory mode 0700).
- **Tokens** live in the OS keyring (`internal/platform` Keyring); the app starts locked and reads the keyring only
  when you unlock it.
- **The activity console** shows every outgoing request and git command with credentials masked
  (`app/activity.go` `MaskSecrets`). Chat message text is never written to it.
- **Outgoing connections** go only to services you configure: your Jira site, Tempo, Google (Chat, and Calendar
  over the API if connected), your HRmaster site, and the ICS feed URLs you add. There is no telemetry.

### Known limits

- Site windows share the app's cookie store, so all profiles' Google sessions are visible to Google's pages in any
  of them, as Google's own multi-sign-in expects.
- Scripts that read Google and HRmaster run in those sites' pages, so those pages can see and alter what the
  scripts return. Their results are treated as untrusted input: parsed, validated and stored as data.
