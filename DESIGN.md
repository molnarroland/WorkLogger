---
name: WorkLogger
description: Your month already happened. WorkLogger writes the timesheet for it.
colors:
  accent-system-blue: "#007AFF"
  state-complete-green: "#34C759"
  state-partial-orange: "#FF9500"
  state-empty-red: "#FF3B30"
  state-leave-purple: "#AF52DE"
  state-extra-blue: "#007AFF"
  state-idle-gray: "#8E8E93"
  source-coding-blue: "#007AFF"
  source-meeting-purple: "#AF52DE"
  source-chat-teal: "#30B0C7"
  source-leave-pink: "#FF2D55"
  source-period-indigo: "#5856D6"
  source-manual-gray: "#8E8E93"
  needs-ticket-orange: "#FF9500"
  label-primary: "#000000D9"
  label-secondary: "#00000080"
  label-tertiary: "#00000040"
  fill-quaternary: "#0000001A"
  window-background: "#FFFFFF"
  sidebar-background: "#EBEBEB"
typography:
  page-title:
    fontFamily: "SF Pro, -apple-system, system-ui"
    fontSize: "17px"
    fontWeight: 600
    lineHeight: 1.3
  big-figure:
    fontFamily: "SF Mono, ui-monospace"
    fontSize: "22px"
    fontWeight: 400
    fontFeature: "tnum"
  headline:
    fontFamily: "SF Pro, -apple-system, system-ui"
    fontSize: "13px"
    fontWeight: 700
    lineHeight: 1.3
  body:
    fontFamily: "SF Pro, -apple-system, system-ui"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.35
  callout:
    fontFamily: "SF Pro, -apple-system, system-ui"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.35
  caption:
    fontFamily: "SF Pro, -apple-system, system-ui"
    fontSize: "10px"
    fontWeight: 400
    lineHeight: 1.3
    fontFeature: "tnum"
rounded:
  swatch: "2px"
  mini-day: "7px"
  day-card: "12px"
  panel: "18px"
  hero-card: "20px"
  pill: "9999px"
spacing:
  hair: "2px"
  xs: "4px"
  sm: "8px"
  md: "12px"
  panel: "14px"
  lg: "16px"
  xl: "24px"
  xxl: "32px"
components:
  button-prominent:
    backgroundColor: "{colors.accent-system-blue}"
    textColor: "#FFFFFF"
    rounded: "{rounded.pill}"
    height: "32px"
    padding: "0 16px"
  button-glass:
    textColor: "{colors.label-primary}"
    rounded: "{rounded.pill}"
    height: "28px"
    padding: "0 12px"
  toolbar-pill:
    textColor: "{colors.label-secondary}"
    rounded: "{rounded.pill}"
    height: "32px"
    padding: "0 12px"
  day-card:
    rounded: "{rounded.day-card}"
    padding: "{spacing.sm}"
    height: "104px"
  glass-panel:
    rounded: "{rounded.panel}"
    padding: "{spacing.panel}"
  sidebar-item-selected:
    backgroundColor: "{colors.accent-system-blue}"
    textColor: "#FFFFFF"
    rounded: "8px"
---

# Design System: WorkLogger

## Overview

**Creative North Star: "The Month-End Desk"**

WorkLogger is a calm, native Mac workspace that you sit down at once a month to close out the timesheet. The desk is quiet: a white window, a gray sidebar and a month grid of day cards. The tools float above it as Liquid Glass: capsule pills in the toolbar, one prominent blue **Log in Tempo** action, and glass panels for summaries. Nothing on the desk is decorative. Every tint means something about a day or a source of work, and every number is set in tabular figures so the columns line up like a ledger.

The system is Apple's, used precisely. Type is SF Pro in the system text styles, colours are the dynamic system colours (so dark mode, Increase Contrast and the user's accent come free), and controls are stock SwiftUI with the macOS 26 glass button styles. The app's own identity lives in a small number of signature pieces: the day card, the source-split progress bar and the fixed meaning of each colour.

Density is moderate to high. A full month fits one window without scrolling at 1600 × 1000. Secondary text carries most of the labelling, so the primary-label figures (durations, day numbers) stand out.

**Key Characteristics:**
- Native first: SF Pro text styles, system semantic colours, SwiftUI controls, Liquid Glass for floating chrome only.
- Colour is a signal, never decoration. One fixed colour per day state and per work source, the same on every screen.
- Orange always means "needs a ticket".
- Tabular, monospaced digits wherever durations or totals appear.
- Glass floats over content. The content itself (grids, lists) stays flat.

## Colors

All colours are SwiftUI system colours (`.blue`, `.green`, …) and adapt to light/dark and contrast settings. The hex values here are the light-mode reference. In code, always use the semantic colour, never a literal.

### Primary
- **System Accent Blue** (`accent-system-blue`): the app has no custom accent. It is used for the prominent **Log in Tempo** button, the selected sidebar row, the selected review filter, today's day number, the drop-target outline on day cards, and the selected-card glass tint (35% opacity).

### Secondary: day states (Timesheet)
A day card is tinted at 10% fill with a 45% stroke of its state colour:
- **Complete Green** (`state-complete-green`): logged equals expected. It also marks Tempo read-back that covers the day.
- **Partial Orange** (`state-partial-orange`): something logged, short of the target.
- **Empty Red** (`state-empty-red`): a past workday with nothing logged.
- **Leave Purple** (`state-leave-purple`): leave or holiday, with the HR label (e.g. "Szabadság") in purple caption text.
- **Extra Blue** (`state-extra-blue`): time logged on a rest day.
- **Idle Gray** (`state-idle-gray`): rest days and upcoming days.

### Tertiary: work sources (bars, legends, totals)
Defined once in `Views/SourceColors.swift`:
- **Coding Blue** (`source-coding-blue`): git commits.
- **Meeting Purple** (`source-meeting-purple`): calendar events.
- **Chat Teal** (`source-chat-teal`): Google Chat stretches.
- **Leave Pink** (`source-leave-pink`): absence entries.
- **Period Indigo** (`source-period-indigo`): period fill.
- **Manual Gray** (`source-manual-gray`): hand-added entries.
- **Needs-a-Ticket Orange** (`needs-ticket-orange`): any time without an issue key, drawn last in the bar.

### Neutral
- **Label Primary / Secondary / Tertiary** (`label-*`): the system label hierarchy. Secondary is the workhorse (about 70 uses) for captions, units, totals and descriptions. Tertiary is for out-of-range day numbers.
- **Quaternary Fill** (`fill-quaternary`): the empty track behind progress bars, and the hairline stroke on out-of-month cells and mini-calendar days.
- **Window White / Sidebar Gray** (`window-background`, `sidebar-background`): system window and sidebar materials, never painted over.

### Named Rules
**The One Meaning Rule.** Each colour has exactly one job per family: a day state, or a work source. A new feature reuses an existing meaning or argues for a new colour. It never borrows a hue for decoration.

**The Orange Alarm Rule.** Orange is reserved for "needs attention": no ticket, a partial day, totals that don't match HR. Never use it for emphasis or branding.

**Known drift (fix in the refresh).** Leave appears in two hues: day cards use purple, but bars and the legend's "Leave" swatch use pink, which reads as red next to "Nothing logged". Meeting purple and leave purple are also the same hue in different roles. Periods (indigo) and manual entries (gray) are drawn in bars but missing from the Timesheet legend.

## Typography

**Display / Body Font:** SF Pro (system), via SwiftUI text styles only.
**Figures:** SF Pro with `.monospacedDigit()`. SF Mono (`.monospaced()`) is used for big totals and issue-key inputs.

**Character:** this is the Mac's own voice, set at standard macOS sizes. The hierarchy comes from weight and the secondary/primary label contrast, not from scale jumps.

### Hierarchy
- **Page Title** (`.title2.weight(.semibold)`, 17pt): page headers ("Review", "2026. October") and the locked screen.
- **Big Figure** (`.title.monospaced()`, 22pt): headline totals on Totals and Export.
- **Headline** (`.headline`, 13pt bold): day numbers in day cards, section titles inside panels, and the centered "WorkLogger" toolbar title.
- **Body** (13pt): list rows and form fields.
- **Callout** (`.callout`, 12pt, about 33 uses): toolbar range text, row metadata, status bar and helper text.
- **Caption / Caption 2** (10pt): per-source durations in day cards, "8h / 8h" totals, HR labels and period chips. Always tabular.

### Named Rules
**The Ledger Digits Rule.** Every duration, time and total uses monospaced digits (`.monospacedDigit()` at minimum), so columns align and numbers don't jitter while editing.

**The System Styles Only Rule.** Use the semantic text styles (`.headline`, `.callout`, `.caption2`) and not fixed point sizes, so Dynamic Type and the Hungarian strings scale together. `.system(size: 44)` is used only for the lock glyph.

## Layout

The window is a three-column `NavigationSplitView`:
- **Sidebar:** about 270pt. Numbered workflow groups ("1 · Set up", "2 · Bring in the month", "3 · Fill the timesheet", "4 · Log in Tempo") in secondary callout text, with SF Symbol + label rows.
- **Content:** the page.
- **Inspector:** optional, about 380pt. Contextual help and legends on the Timesheet, and the selected day's entries (a popover on narrow windows).

A thin status bar at the bottom shows the last action on the left and attention counts on the right ("1 needs a ticket · 38 of 69 ticket-days in Tempo").

- **Toolbar:** range stepper and presets as standalone glass pills on the left, the title in the centre, and on the right the Collect menu, the view switcher, the prominent Log in Tempo button, Export and Settings.
- **Timesheet grid:** a 7-column grid with Monday first. The weeks share the grid's height evenly (at least 56pt each), with a 6pt gutter, so the last week never runs under the status bar. A card that is too short or narrow for its source rows shows only the day number, the bar and the total.
- **Window sizes:** the window works from 700 × 560pt (half-screen tiling) up. The page never gets narrower than 500pt: below that the split view folds the sidebar away. Under 980pt the toolbar drops the centered title and Log in Tempo becomes icon-only. Under 900pt the Timesheet's day panel opens as a popover on the clicked day, and the legend moves behind an info button. Side columns (Meetings, By ticket) shrink to about 210–260pt. Periods stacks its month under the list below 760pt.
- **Page headers wrap:** `PaneHeader` puts the title and controls on one row when they fit. Otherwise the controls move to a second row and wrap (`FlowLayout`). Every page header uses it, so long Hungarian labels never shrink to "…".
- **Rhythm:** a 4pt base. 4pt is used inside cards, 8pt for card padding and grid gaps, 12pt for page-header padding, 14–16pt for glass-panel padding, 24pt and 32pt for welcome and empty-state composition.
- **Page headers:** title, then a secondary count, then the controls trailing. They sit 12pt from the content edge.

## Elevation & Depth

Depth is layered, not shadowed. There are two planes. The **content plane** is flat: the grid, lists and tinted day cards sit on the window background with no shadows. The **glass plane** floats above it: toolbar pills, buttons, summary panels, the Export sheet and drag previews use Liquid Glass (`.glassEffect(.regular)`, often `.interactive()`). These pick up the content behind them through the system material. No custom `shadow()` exists anywhere in the app.

### Named Rules
**The Glass Floats Rule.** Glass is for things that act or summarise above the work: controls, panels and drag previews. Data cells and list rows are never glass. The one exception is a *selected* day card, which gains an accent-tinted interactive glass so that it lifts out of the grid.

**The No Custom Shadow Rule.** Depth comes from the system material and tint, not from drop shadows.

## Shapes

Shapes are soft and continuous, sized to the object:
- **Capsule** for anything you press or carry: toolbar pills, buttons, period chips, progress bars and drag previews.
- **12pt** rounded rect for day cards.
- **18pt** for glass panels (Totals, Periods, Reports).
- **20pt** for the two large welcome cards.
- **7pt** for mini-calendar days.
- **2pt** for 10 × 10 legend swatches.

Strokes are 1pt hairlines (state colour at 45%, or quaternary). Drop targets and focus use a 2pt accent stroke.

## Components

### Buttons
- **Prominent:** `.buttonStyle(.glassProminent)`, accent-blue glass capsule, white label with leading SF Symbol. There is exactly one per window context (**Log in Tempo**, **Unlock**), at `.controlSize(.large)` on the locked screen.
- **Glass:** `.buttonStyle(.glass)`, the default for page actions (about 23 uses): filters, "This month", "Fetch range", "Save changes".
- **Borderless:** inline row actions ("Mark reviewed", "+ Add entry", the lock toggle) and buttons inside a glass pill that already supplies the glass.
- **Disabled:** system dimming only (e.g. "Save changes" with nothing to save).

### Toolbar Pill
A standalone glass capsule, 32pt tall with 12pt horizontal padding and `.regular.interactive()` glass (`toolbarPill()` in `ContentView.swift`). It holds the range stepper ("‹ 2026. Oct 1–31. ›") and the presets menu. Its contents use borderless buttons.

### Chips / Filters
- **Review filters:** glass buttons in a row ("No ticket · 1", "Drafts", "Locked", "All"). The selected one takes the accent tint. The count is part of the label.
- **Period chip:** caption2 medium text on a capsule of the period's colour at 25%, inside the day card.

### Glass Panel
A `VStack` of headline title + content, 14pt padding, `.glassEffect(.regular, in: .rect(cornerRadius: 18))`. Used for Totals breakdowns, Periods' mini-calendars and summary blocks.

### Navigation (Sidebar)
`.listStyle(.sidebar)` with numbered workflow section headers in secondary text. Each row is a multicolour SF Symbol + body label. The selected row is the system accent capsule with white text. ⌘1–⌘9 map to the rows.

### Day Card (signature)
The heart of the Timesheet:
- **Header:** day number (headline; accent colour when today), a small orange warning triangle when any entry lacks a ticket, and the HR label trailing in purple caption2 on non-workdays.
- **Body:** an optional period chip, then up to three rows of "source icon … duration" in caption2 tabular figures. Each row can be dragged to another day.
- **Read-back line:** "Tempo 6h" with a seal icon. It turns green once Tempo covers the logged time.
- **Footer:** a source stack bar and "logged / expected" in secondary caption2.
- **Shape and fill:** a 12pt card filled with the state colour at 10% with a 45% hairline. When selected it gets accent-tinted interactive glass, and as a drop target a 2pt accent outline. Out-of-month days show only a tertiary day number at 50% opacity with a quaternary hairline.

### Source Stack Bar (signature)
A 5pt capsule (`SourceStack`). The track is quaternary, the width is the day's target, and the bar never draws shorter than the logged total. It is filled left to right with each source's colour in proportion, with needs-a-ticket orange last. The same component, taller, shows month totals.

## Do's and Don'ts

### Do:
- **Do** use system semantic colours (`.blue`, `.secondary`, `.quaternary`), never hex literals, so light/dark and contrast settings keep working.
- **Do** colour a day card by its state at 10% fill and 45% stroke, and keep the state set fixed: complete, partial, empty, leave, extra, idle.
- **Do** take source colours from `WorklogSource.color` and draw time without a ticket in orange, last in the bar.
- **Do** put `.monospacedDigit()` on every duration, time and total.
- **Do** use `.glass` for page actions and `.glassProminent` for the single primary action.
- **Do** keep chrome in glass capsules (`toolbarPill()`) and content flat.
- **Do** size layouts for Hungarian strings, which run noticeably longer than English.

### Don't:
- **Don't** add drop shadows. Depth comes from Liquid Glass and tint.
- **Don't** use orange for anything except "needs attention".
- **Don't** make list rows or grid cells glass, except the selected day card.
- **Don't** add a second prominent button to a view.
- **Don't** use fixed font sizes where a system text style fits.
- **Don't** introduce new leave hues. Leave is currently split between purple (cards) and pink (bars), and the refresh should settle on one.
