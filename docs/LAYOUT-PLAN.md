# Layout plan: Days / Sources side panel and guided setup

Proposal with mockups: https://claude.ai/artifact/KYQfyYEmXUuvHE1FKajk4F

## Why

The Days list, the main area and the details panel work. The rest confuses people:

- **"All pages" mixes everything.** 13 pages in one launcher, grouped by step names ("Set up", "Bring in the month")
  that don't say what the pages are.
- **Setup is spread out.** The welcome sheet, Settings → Profiles (checklist), Repositories and Settings → Calendars.
- **Pages don't say whether a source works.** An empty Work sessions page can mean module off, HRmaster not signed in,
  or not collected yet.
- **Two names for one thing.** Repositories / commits, Work times / Work sessions / HRmaster.

## Phase 1: side panel switcher (done)

A segmented **Days | Sources** switcher at the top of the side panel replaces "All pages" and pinning.

| Tab     | Group                         | Pages                                                             |
| ------- | ----------------------------- | ----------------------------------------------------------------- |
| Days    | (top)                         | Overview, Days, Week                                              |
| Days    | Needs attention / Drafts      | the day lists, unchanged                                          |
| Days    | Month                         | Timesheet, Review, By ticket (Log in Tempo), Totals               |
| Sources | Where your time comes from    | Repositories, Meetings, Work sessions, Chats, Jira, Confluence, Mail |
| Sources | Your calendar                 | Periods, Plans                                                    |
| Sources | (bottom)                      | Finish setup (n of m), Add or remove sources…                     |

- `Pane.place` (`view | month | source | calendar`) replaces `Pane.view`; `STEPS` goes. `Pane.step` stays for the
  welcome sheet's flow.
- A source's badge is its month count, or "Set up" when one of its setup steps (`Api.setupSteps`) isn't done.
  The Sources pill shows how many setup steps are left.
- Opening a page switches to the tab that holds it, so the current page is always visible.
- The chosen tab is remembered per window (`wl.sidebar.tab`).
- Folded panel: the switcher stacks as two icon buttons.
- `ui/page-launcher` and pinning (`wl.sidebar.pinned`) are deleted.
- The source dock under the page stays for now (it counts the open day). Revisit after phase 2.

## Phase 2: source pages own their setup (done)

- The shell gives every source page an **Activity | Setup** switch (`?tab=setup`); Setup is
  `shell/source-setup`, Activity the page itself.
  - Repositories: folders, commit emails, skipped folders (`pages/repositories/repo-setup`, moved off the page).
    The Jira-project and per-repository ticket rules stay on Activity: they decide where commits go.
  - Meetings: calendars (`calendar-settings`; the Settings → Calendars tab is gone).
  - Work sessions, Chats, Jira, Confluence, Sent mail: that kind's profile form (`profiles-settings [only]`).
  - Each tab starts with the source's setup steps, ticked when done.
- Activity shows the first setup step left as a warning with a **Set up** button.
- `toSetupSteps` sends a source's steps to its Setup tab. Google and Jira stay on Settings → Profiles (more than
  one source uses them), Tempo too. `SETUP_PANE` in `shared/setup-steps.ts` is the one map.
- Settings keeps Profiles (all accounts), Modules, Timesheet, Coding, Chats, General.

Left for later:
- Settings → Chats (chat gap, chat rules) could move to Chats → Setup.
- "Not collected yet" empty state (needs to know whether Collect ran for the month).

## Phase 3: guided first run (done)

`shell/onboarding` (shown until `settings.welcomed`):
Welcome → What do you use? → one page per "yes" → First Collect → first day to do.

- Welcome: the month's three steps (Collect, Check each day, Log in Tempo). Setup is one-off, so it isn't one of them.
- "Step n of m" on every page after Welcome.
- Repositories page reuses `repo-setup` (the Repositories → Setup form); an empty commit-email list uses
  `git config --global user.email` on the first Collect (the backend already did this).
- Account pages are the same `profiles-settings [only]` forms as the Setup tabs, with **Skip for now**.
- Last page runs Collect for the month on screen: each source with what it brought in, how many days need you,
  a retry on failure, and the setup steps left for later. Its button saves `welcomed` and opens the first day that
  needs work (the overview when none). It stays disabled until something is set up.

Left for later:
- `ui/welcome-sheet` (Setup guide…, Finish setup, Modules…) is still separate. Merge it into the onboarding so
  Finish setup reopens the onboarding at the first step left.
- "Show more" for the less common sources (Confluence, Sent mail) on the questions page; today they're only
  switched on in Settings → Modules.
- A failing source on the Collect page has no per-source fix button: the job reports one error for the run.

## Phase 4: main area polish (done)

- **Collect** shows its label in the header, not only an icon.
- **Header:** a page with its own day or week pager (Days, Week) shows ‹ Today › and the picker; the month
  « » buttons show only on pages without one. The month stays on ⌘[ / ⌘] and in the menu.
- **Overview** opens with **Next up**: the first day that needs work, what it needs, and Open day.
  `dayNote` (shared/worklogs.ts) is the side panel's line under a day, used by both.
- **Details panel:** an entry without a ticket shows the ticket field first (CSS `order`, no second copy).
- **Log in Tempo:** one side panel item (By ticket) with **By ticket | Totals** tabs. `Pane.parent` makes a page a
  tab of another; `Pane.sideLabel` names it in the side panel.
- Naming: "Fetch work sessions from HRmaster" (was "work times").

Decided against:
- **Merging Review into Days.** Review has bulk ticket editing across the month; Days doesn't. Kept as is.
- **Renaming Repositories → Commits.** The page lists repositories (with their commits), so the name fits.

Left for later:
- Collect on a source page collecting only that source (needs per-source jobs in the backend; HRmaster and Chats
  have them, git and calendar don't).
- Ticket suggestions as chips in the details panel (Days → What happened already offers "Use KEY").
- ⌘1 / ⌘2 for the side panel tabs.
