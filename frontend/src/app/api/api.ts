import { InjectionToken } from '@angular/core';
import type {
  AccentColor,
  Appearance,
  Contrast,
  Motion,
  SkinChoice,
  TextScale,
  Transparency,
} from '../shared/appearance';
import type { LanguageChoice } from '../i18n/t.pipe';

// The DTOs mirror the Go DTOs (app/dto.go, internal/core). WailsApi casts the generated `@bindings/*` types to
// these (they differ only in string unions vs plain strings); bindings-check.ts fails the build when a field differs.
// After a Go change: `task bindings`.
export type Module = 'git' | 'calendar' | 'workTimes' | 'chats' | 'jira' | 'confluence' | 'email';
export type Source = 'git' | 'jira' | 'confluence' | 'email' | 'meeting' | 'absence' | 'manual' | 'period' | 'chat';
/** `exported` covers both sent by the Tempo API and ticked off after typing it into Tempo by hand. */
export type Status = 'draft' | 'reviewed' | 'exported';

export type RoundingMode = 'estimate' | 'fixedHours' | 'sessions';
export type WorklogProvider = 'tempo' | 'jira' | 'other';

export interface MeetingRule {
  id: string;
  contains: string;
  issueKey: string;
}

/** A conversation always logged to one ticket (Chats → "Always"). */
export interface ChatRule {
  conversationId: string;
  name: string;
  issueKey: string;
}

/** A stretch of days booked to one issue. Day keys, inclusive. */
export interface ProjectPeriod {
  id: string;
  title: string;
  issueKey: string;
  from: string;
  to: string;
  /** Goes to Tempo with the time the period fills in. */
  comment: string;
}

/**
 * What a profile connects to. Jira profiles are the `jiraAccounts`; the other kinds are `profiles`. Several
 * profiles of a kind are all read and merged.
 */
export type ProfileKind = 'googleChat' | 'hrmaster' | 'jira' | 'tempo' | 'confluence' | 'email';

/** A Google account (Chat, and Calendar without the API), an HRmaster site, Tempo for one Jira profile, or Confluence. */
export interface Profile {
  id: string;
  kind: Exclude<ProfileKind, 'jira'>;
  name: string;
  /** Google: the account's address (picks it in Google's multi-sign-in; "me" in chats). */
  email: string;
  /** HRmaster: the company's address. Tempo: the API base URL. Confluence: the site, when not the linked Jira's. */
  url: string;
  /**
   * Tempo: the Jira profile whose issues it logs; its Tempo token is kept under that account.
   * Confluence: the Jira profile whose site, email and token it uses ("" for none); its own, when given, win.
   */
  jiraAccountId: string;
}

/** A Jira profile's Jira or Tempo token, or a Confluence profile's own token (by the profile's id). */
export type TokenKind = 'jira' | 'tempo' | 'confluence' | 'email';

/** The saved settings after Api.addProfile, with the new profile's id. */
export interface ProfileEdit {
  settings: SettingsDto;
  id: string;
  /** A Tempo profile was asked for, but every Jira profile already has one: nothing was added. */
  needsJira: boolean;
}

/** One Jira site with its own sign-in and worklog provider. Tokens live in the keyring, not here. */
export interface JiraAccount {
  id: string;
  name: string;
  baseURL: string;
  email: string;
  accountId: string;
  provider: WorklogProvider;
  tempoBaseURL: string;
  projectKeys: string[];
}

/**
 * The user's settings (core.Settings). Keys kept only for older databases (macCalendarIds, knownModules) stay in Go.
 */
export interface SettingsDto {
  modules: Module[];
  tempoByHand: boolean;
  welcomed: boolean;
  /** The first of workspaceRoots (kept for older databases). */
  workspaceRoot: string;
  /** Every folder scanned for repositories; the skipped folders apply to all of them. */
  workspaceRoots: string[];
  authorEmails: string[];
  excludedFolders: string[];
  ignoredPathPatterns: string[];
  minCommitMinutes: number;
  maxCommitMinutes: number;
  maxCommitGapMinutes: number;
  roundingMode: RoundingMode;
  fixedDailyHours: number;
  granularityMinutes: number;
  dayRoundUpMinutes: number;
  defaultDayStart: string;
  defaultDayEnd: string;
  includeWeekends: boolean;
  language: LanguageChoice;
  appearance: Appearance;
  accentColor: AccentColor;
  /** The rest of the look; "system" (or textScale 0) follows the OS. */
  skin: SkinChoice;
  transparency: Transparency;
  contrast: Contrast;
  motion: Motion;
  textScale: TextScale;
  chatGapMinutes: number;
  chatRules: ChatRule[];
  /** The first Google / HRmaster profile's address (kept for older databases; the backend writes them). */
  chatEmail: string;
  hrmasterURL: string;
  profiles: Profile[];
  absenceIssueKey: string;
  absenceRules: MeetingRule[];
  periods: ProjectPeriod[];
  googleClientId: string;
  googleCalendarId: string;
  icsUrls: string[];
  defaultMeetingIssue: string;
  meetingRules: MeetingRule[];
  skipMeetingKeywords: string[];
  jiraAccounts: JiraAccount[];
  /** Always an existing Jira profile (the first when none was picked), null when there is none. */
  defaultJiraAccountId: string | null;
}

export type SetupStepId =
  'repositories' | 'google' | 'calendar' | 'hrmaster' | 'jira' | 'confluence' | 'email' | 'tempo';

/** Something the chosen modules still need: done on a profile of kind `profile`, else on `page` (with `tab`). */
export interface SetupStepDto {
  id: SetupStepId;
  done: boolean;
  profile: ProfileKind | '';
  page: string;
  tab: string;
}

export interface RepoDto {
  id: number;
  path: string;
  name: string;
  issueKey: string | null;
  projectKey: string | null;
  enabled: boolean;
  /** Commits by you in the selected month. */
  commits: number;
  /** Those commits by day ("YYYY-MM-DD" → count); days without any are left out. */
  days: Record<string, number>;
}

/** A Jira project's catch-all issue, for repositories assigned only to a project. */
export interface ProjectDto {
  key: string;
  defaultIssue: string | null;
}

export interface SessionDto {
  id: number;
  day: string;
  start: string;
  end: string;
  kind: 'work' | 'pause';
  label: string;
  /** Minutes since the day's local midnight (past 1440 when it runs over). */
  startMinute: number;
  endMinute: number;
}

/** One day of the Work sessions page: its sessions next to HR's record (HRmaster's Munkaidő calendar). */
export interface SessionDayDto {
  day: string;
  label: string;
  status: 'workday' | 'restday' | 'holiday' | 'leave';
  /** By start time. */
  sessions: SessionDto[];
  /** Work sessions minus breaks, seconds. */
  net: number;
  /** HR's "Ledolgozott órák", seconds; 0 when unknown. */
  reported: number;
  /** Both known and more than a minute apart. */
  mismatch: boolean;
}

export interface MeetingDto {
  id: string;
  /** The day it starts. */
  day: string;
  /** Its length. */
  seconds: number;
  title: string;
  start: string;
  end: string;
  /** Unticked by hand. */
  skipped: boolean;
  /** Left out by a "never log" keyword. */
  skippedByKeyword: boolean;
  /** Issue from the title, a rule or the default; null when none applies. */
  issueKey: string | null;
}

/** A range's meetings, and the untagged title worth a rule (null: none comes up twice). */
export interface MeetingsDto {
  meetings: MeetingDto[];
  suggestion: { title: string; count: number } | null;
}

export interface CalendarStatus {
  googleConnected: boolean;
  /** Without the API: read from calendar.google.com with the Google Chat window's sign-in. */
  googleWeb: boolean;
}

/** One conversation's messages on one day. */
export interface ChatDayDto {
  id: string;
  day: string;
  conversationId: string;
  name: string;
  type: 'dm' | 'space';
  count: number;
  /** Stretches of chatting, seconds. */
  activeSeconds: number;
  first: string;
  last: string;
  /** Issue of the worklog made from it, if any. */
  issueKey: string | null;
  /** That worklog is already in Tempo. */
  exported: boolean;
  /** The issue an "Always" rule logs the conversation to. */
  ruleIssue: string | null;
}

/** A range's conversation-days under their days, each day with its active time. */
export interface ChatsDto {
  days: { day: string; seconds: number; chats: ChatDayDto[] }[];
  active: number;
}

export interface ChatMessageDto {
  time: string;
  sender: string;
  me: boolean;
  text: string;
  files: string[];
}

/** A profile id: Google and HRmaster profiles each sign in in their own window. */
export type WebSource = string;

export interface CheckLine {
  title: string;
  detail: string;
  /** null: skipped */
  ok: boolean | null;
}

export type ExportFormat = 'markdown' | 'csv' | 'json';

export interface WorklogDto {
  id: number;
  /** "YYYY-MM-DD" */
  day: string;
  /** RFC 3339 */
  start: string;
  /** RFC 3339; derived, read-only. */
  end: string;
  /** seconds */
  duration: number;
  issueKey: string | null;
  /** WorkLogger's own note (commit messages, meeting title). */
  description: string;
  /** What goes to Tempo; required before the entry can be marked reviewed. */
  comment: string;
  source: Source;
  /** Raw estimate before the day was scaled to its target, seconds. */
  estimated: number;
  status: Status;
  /** Kept when drafts are rebuilt. Editing an entry locks it. */
  locked: boolean;
  tempoId: number | null;

  // Derived by the backend (internal/view); read-only, ignored when saving.
  /** Has time but no issue: Tempo can't take it, so exports skip it. */
  needsIssue: boolean;
  /** A draft without its comment for Tempo (needed before Reviewed). */
  needsComment: boolean;
  /** What Tempo gets: the comment, else the description. */
  tempoText: string;
  /** Exported, or Tempo already has its ticket-day (read-back). */
  inTempo: boolean;
  /** `exported` when in Tempo, else `status`. */
  effectiveStatus: Status;
  /** Not sent through the Tempo API. */
  canDelete: boolean;
  /** Not exported: may be dragged to another day. */
  canMove: boolean;
  /** Minutes since local midnight. */
  startMinute: number;
  /** Where the entry's source can be seen; null for an entry made by hand. */
  focus: Focus | null;
  /** The ticket Tempo planned the most time for on the day, suggested while the entry has none. */
  planIssue: string | null;
}

/** What sending a range to Tempo would do. */
export interface PushPreviewDto {
  /** The reviewed entries that would be sent, their time, and how many days they are on. */
  entries: number;
  seconds: number;
  days: number;
  /** Days left out because they still have drafts. */
  draftDays: string[];
  /** Reviewed entries on days that haven't come yet, left out. */
  later: number;
}

/** A stored commit behind a git entry, with its repository's name. */
export interface CommitDto {
  repoId: number;
  repo: string;
  hash: string;
  /** RFC 3339. */
  at: string;
  author: string;
  subject: string;
  body: string;
  /** The branch it was reached from. */
  ref: string;
  files: number;
  insertions: number;
  deletions: number;
}

/** A file a commit changed; a rename's path is git's `old => new`. */
export interface CommitFileDto {
  path: string;
  added: number;
  deleted: number;
}

/** A commit's patch; `truncated` when it was too long to show whole. */
export interface CommitDiffDto {
  text: string;
  truncated: boolean;
}

/**
 * A ticket offered for an entry without one. `origin`: planned in Tempo for the day, used by another entry that
 * day, a project period's, used in this period or an earlier one, or found by a Jira search.
 */
export interface IssueOptionDto {
  key: string;
  summary: string;
  origin: 'plan' | 'day' | 'period' | 'recent' | 'jira';
  /** Last used (`day`, `recent`) or created (`jira`); '' otherwise. */
  day: string;
  uses: number;
  status: string;
  /** The start of the issue's description, on one line (`jira`); '' otherwise. */
  description: string;
  /** The Jira profile a search found it with (openJiraIssue); '' otherwise. */
  account: string;
  /** That profile's name when there are several, else ''. */
  accountName: string;
}

/** One of WorkLogger's entries next to a plan (its commits, chats, meetings…). */
export interface PlanEntryDto {
  id: number;
  source: Source;
  /** What Tempo gets. */
  description: string;
  seconds: number;
  status: Status;
  focus: Focus | null;
}

/**
 * A ticket-day Tempo has: `plan` is time Tempo Planner planned for you (Jira or Tempo adds it when a ticket is
 * assigned or changes status); `tempo` a worklog Tempo has that WorkLogger didn't send.
 */
export interface PlanItemDto {
  kind: 'plan' | 'tempo';
  key: string;
  day: string;
  seconds: number;
  description: string;
  /** WorkLogger's time on the same ticket and day (the entries'). */
  logged: number;
  entries: PlanEntryDto[];
}

export interface PlanDayDto {
  day: string;
  /** The day's planned time (worklogs found in Tempo don't count). */
  planned: number;
  items: PlanItemDto[];
  /** The day's entries without a ticket, and the planned ticket suggested for them. */
  untagged: PlanEntryDto[];
  suggest: string | null;
}

/** A field you changed on a Jira ticket ("15:04", the field as Jira names it, old and new value). */
export interface JiraChangeDto {
  time: string;
  field: string;
  from: string;
  to: string;
}

/** A comment you wrote, or a file you attached (text is its name). */
export interface JiraNoteDto {
  time: string;
  text: string;
}

/** What you did on one Jira ticket in one day. */
export interface JiraTicketDto {
  /** The Jira profile's id (openJiraIssue), and its name when there are several profiles (else ''). */
  account: string;
  accountTitle: string;
  issueKey: string;
  summary: string;
  /** The day's first and last activity on it ("15:04"). */
  first: string;
  last: string;
  created: boolean;
  statuses: JiraChangeDto[];
  /** Other fields than status and attachments. */
  changes: JiraChangeDto[];
  comments: JiraNoteDto[];
  attachments: JiraNoteDto[];
  /** The day's entries without a ticket that ran around this ticket's activity: likely its work. */
  suggested: PlanEntryDto[];
}

export interface JiraDayDto {
  day: string;
  tickets: JiraTicketDto[];
}

/** How much each source page has for a day or period (the bottom bar's badges), keyed by the page's id. */
export interface SourceCountsDto {
  repositories: number;
  meetings: number;
  sessions: number;
  chats: number;
  jira: number;
  confluence: number;
  email: number;
}

/** The mail you sent in one day on one subject (Re:/Fwd: aside). */
export interface EmailThreadDto {
  subject: string;
  /** When you sent each ("15:04"), and each one's own subject, in the same order. */
  times: string[];
  subjects: string[];
  /** A ticket key in the subject (its time goes to that ticket), else ''. */
  issueKey: string;
  /** The draft or entry the thread-day became (its suggested time). */
  entry: PlanEntryDto | null;
}

export interface EmailDayDto {
  day: string;
  threads: EmailThreadDto[];
}

/** The collected sent mail by day (newest first), its size, and how many Email profiles there are (0: none). */
export interface EmailsDto {
  days: EmailDayDto[];
  mails: number;
  threads: number;
  profiles: number;
}

/** What you did on one Confluence page in one day. */
export interface ConfluencePageDto {
  /** The Confluence profile's id (openConfluencePage), and its name when there are several profiles (else ''). */
  profile: string;
  profileTitle: string;
  pageId: string;
  /** page or blogpost (confluenceChanges). */
  kind: string;
  title: string;
  space: string;
  /** The day's first and last activity on it ("15:04"). */
  first: string;
  last: string;
  /** When you saved a version of it ("15:04"); what you commented on it, and when. */
  edits: string[];
  comments: JiraNoteDto[];
  /** A ticket key in the title (its work goes to that ticket), else ''. */
  issueKey: string;
  /** The draft or entry the page-day became (its suggested time), once collected. */
  entry: PlanEntryDto | null;
  /** The day's entries without a ticket that ran around it, when it has a key. */
  suggested: PlanEntryDto[];
}

/** One version you saved of a page: how many lines it added and removed against the one before, and the first few. */
export interface ConfluenceChangeDto {
  time: string;
  number: number;
  message: string;
  added: number;
  removed: number;
  addedText: string[];
  removedText: string[];
}

export interface ConfluenceDayDto {
  day: string;
  pages: ConfluencePageDto[];
}

/** The Confluence feed by day (newest first), its size, and how many Confluence profiles it was read from. */
export interface ConfluenceActivityDto {
  days: ConfluenceDayDto[];
  entries: number;
  pages: number;
  profiles: number;
}

/** The Jira activity feed by day (newest first), its size, and how many Jira profiles it was read from (0: none). */
export interface JiraActivityDto {
  days: JiraDayDto[];
  entries: number;
  tickets: number;
  accounts: number;
}

/** The month's Tempo plans by day, from the last read-back. */
export interface PlansDto {
  days: PlanDayDto[];
  planned: number;
  /** Tempo has been read back for the month; until then there is nothing to show. */
  read: boolean;
}

/**
 * A page and the thing on it: `pane` is the page's id, `day` narrows it to a day, `id` picks one item (a repository
 * id, a calendar event id, a chat-day id). Empty strings mean the page as a whole. Opened by `openFocus`.
 */
export interface Focus {
  pane: 'repositories' | 'meetings' | 'chats' | 'sessions' | 'periods';
  day: string;
  id: string;
}

/**
 * Where a day stands in the review: past and empty, past and short, has time without a ticket, full with drafts,
 * done; or nothing to do (upcoming, free, absence).
 */
export type DayState = 'empty' | 'partial' | 'noTicket' | 'draft' | 'done' | 'upcoming' | 'free' | 'absence';
/** How full a day is against its target (the day cards' colour). */
export type DayProgress = 'absence' | 'free' | 'extra' | 'complete' | 'upcoming' | 'empty' | 'partial';

/** A stacked-bar segment: a source's time, or the time without a ticket (`noTicket`). */
export interface PartDto {
  kind: Source | 'noTicket';
  seconds: number;
}

export interface DayDto {
  day: string;
  /** What the day should add up to, seconds (HR's worked hours, or the fixed hours on a workday). */
  expected: number;
  /** HR label, e.g. "Szabadság" or "Munkanap (Home Office)". */
  label: string;
  /** Leave or public holiday: never filled with work. */
  absence: boolean;
  logged: number;
  /** Target left: expected minus logged, never below zero. */
  short: number;
  /** Entries without a ticket. */
  missing: number;
  /** Drafts that still need a comment for Tempo. */
  noComment: number;
  entries: number;
  drafts: number;
  state: DayState;
  progress: DayProgress;
  /** Entries may be dropped on it. */
  acceptsWork: boolean;
  /** Offered in "Move to…": a workday that takes work. */
  moveTarget: boolean;
  /** The project period booking it, -1 for none. */
  periodIndex: number;
  /** Each source's ticketed time, then the time without a ticket. */
  parts: PartDto[];
  /** Logged time per source, largest first (time without a ticket counted with its source). */
  sources: { source: Source; seconds: number }[];
  /** The next day in the review queue, wrapping; null when nothing else is queued. */
  next: string | null;
  /** The calendar neighbours. */
  prevDay: string;
  nextDay: string;
}

export interface SummaryDto {
  logged: number;
  expected: number;
  /** Still to log this month: expected minus logged, never below zero. */
  short: number;
  /** Target left on past workdays (today included): what's late, not what's upcoming. */
  behind: number;
  behindDays: string[];
  inTempo: number;
  /** Days whose entries are all in Tempo. */
  inTempoDays: number;
  /** Logged but not in Tempo yet (drafts and reviewed). */
  toLog: number;
  draftDays: string[];
  /** Ids of the entries without a ticket, oldest first, and their days. */
  missing: number[];
  missingDays: string[];
  missingSeconds: number;
  /** Logged time per source. */
  parts: PartDto[];
  /** The same, with the time without a ticket split off, last. */
  barParts: PartDto[];
}

export type TodoKind = 'fillDays' | 'addTickets' | 'reviewDrafts' | 'logInTempo';

/** One row of the Overview's "What's left". */
export interface TodoDto {
  kind: TodoKind;
  count: number;
  seconds: number;
  /** Where its action opens; null for the By ticket page. */
  day: string | null;
}

export interface TicketDayDto {
  key: string | null;
  day: string;
  seconds: number;
  description: string;
  ids: number[];
  /** All of the day's entries are in Tempo (sent, ticked off by hand, or found by read-back). */
  entered: boolean;
  /** Tempo's read-back has it, so it can't be ticked off by hand. */
  found: boolean;
  /** Tempo Planner's time for the ticket on the day, seconds (0: none, or not read back yet). */
  planned: number;
}

export interface TicketGroupDto {
  /** null: "No issue". */
  key: string | null;
  days: TicketDayDto[];
  seconds: number;
  enteredDays: number;
  /** Time of the entered days, and the rest. */
  enteredSeconds: number;
  remainingSeconds: number;
  /** A ticket whose days are all in Tempo. */
  done: boolean;
  /** Plain list for a note or for Claude. */
  text: string;
}

export type Period = 'day' | 'week' | 'month';

/** The time of one day, ISO week or month (Totals). */
export interface ReportGroupDto {
  /** "2026-03-02", "2026-W10" or "2026-03" */
  id: string;
  /** The day its label is made from: the day, the week's Monday, or the month's first. */
  start: string;
  total: number;
  /** Largest first; null: "No issue". */
  byIssue: { issue: string | null; seconds: number }[];
  bySource: { source: Source; seconds: number }[];
}

export interface MonthDto {
  month: string;
  /** The backend's today, and its month. */
  today: string;
  currentMonth: string;
  from: string;
  to: string;
  prev: string;
  next: string;
  /** Where the Days view opens (the first queued day, else today in the month, else its first day). */
  openDay: string;
  /** Where the Week view opens (today in the month, else its first day). */
  anchorDay: string;
  /** Calendar grid, Monday first, padded with the neighbouring months' days. */
  weeks: string[][];
  /** Every day of the month. */
  days: DayDto[];
  /** By start time. */
  worklogs: WorklogDto[];
  /** "KEY|day" ticket-days that Tempo already has (read-back). */
  inTempo: string[];
  /** The month's days in weeks Tempo has in full: Collect skips them, and they can be cleaned up. */
  settled: string[];
  summary: SummaryDto;
  todos: TodoDto[];
  /** Past days short of time, and any day with an entry that has no ticket. */
  attention: string[];
  /** Full days with drafts left. */
  drafts: string[];
  /** The order "Mark reviewed and go to next" walks: attention first, then drafts. */
  queue: string[];
  /** "No issue" first, then tickets by time. */
  tickets: TicketGroupDto[];
  /** Ticket-days in Tempo; "No issue" doesn't count. */
  tempoProgress: { entered: number; total: number };
  needsTicket: number;
  /** The Totals page's groups for each period. */
  reports: Record<Period, ReportGroupDto[]>;
  /** What the export sheet sends for each setting of its switches. */
  exports: ExportChoiceDto[];
}

/**
 * What the export sheet would send for one setting of its switches: the entries Tempo can take, their time, and
 * how many are left out for want of a ticket.
 */
export interface ExportChoiceDto {
  onlyReviewed: boolean;
  includeExported: boolean;
  ids: number[];
  seconds: number;
  missing: number;
}

/** Monday to Sunday around a day, across a month end if it falls on one. */
export interface WeekDto {
  day: string;
  /** Monday to Sunday. */
  days: DayDto[];
  worklogs: WorklogDto[];
  logged: number;
  expected: number;
  /** The same weekday a week earlier and later. */
  prev: string;
  next: string;
  /** The week's month as ISO weeks count it (its Thursday's). */
  month: string;
}

/**
 * Something the backend tells the user: an i18n code ("msg.<code>") with its parameters, and the English text,
 * shown as is when there is no code. User errors carry one as the rejection's `cause`.
 */
export interface BackendMessage {
  code: string;
  params?: Record<string, string | number>;
  /** What a job left out and why, as the tools reported it (not translated). */
  skipped?: string;
  text: string;
}

export interface CollectProgress {
  source: string;
  message: BackendMessage;
}

/** collect:done: how a collect, fetch, rebuild or push ended. */
export interface CollectDone {
  cancelled: boolean;
  /** The job ended with an error (message says which). */
  failed: boolean;
  /** The summary for the status bar ("Collected 18 commits…", or what failed). */
  message?: BackendMessage;
  /** Days Collect left out because Tempo has their weeks in full (Collect again forces them). */
  settled?: number;
}

/** What cleanUpSettled removed. */
export interface CleanupDto {
  days: number;
  rows: number;
}

/** activity:line: one line of the activity console (masked by the backend). */
export interface ActivityLine {
  time: string;
  source: string;
  level: 'info' | 'progress' | 'error';
  message: string;
  /** Takes the place of the line before it (the same job's previous progress). */
  replace: boolean;
}

export const EVENTS = {
  collectProgress: 'collect:progress',
  collectDone: 'collect:done',
  /** A fetched Google Chat day is saved (data: the day), while the next one is read. */
  chatDay: 'chat:day',
  activity: 'activity:line',
} as const;

export interface Api {
  /** Reads the keyring item. False when the user declined or it couldn't be read. */
  unlock(): Promise<boolean>;
  isLocked(): Promise<boolean>;
  settings(): Promise<SettingsDto>;
  /** What the chosen modules still need, in the order to do it. */
  setupSteps(): Promise<SetupStepDto[]>;
  /** An empty month is the backend's current one. */
  month(month: string): Promise<MonthDto>;
  /** An empty day is today. */
  week(day: string): Promise<WeekDto>;
  /**
   * Reads back what Tempo (or Jira) already has for the month. Resolves to whether that changed the month view
   * (then reload it); rejects when the read failed. `month` never goes to the network itself.
   */
  refreshTempo(month: string): Promise<boolean>;
  /** The month's Tempo plans from the last read-back, next to the entries on the same ticket-days. No network. */
  plans(month: string): Promise<PlansDto>;
  /** What you did in Jira over the days (inclusive), with every Jira profile. Goes to Jira each time. */
  jiraActivity(from: string, to: string): Promise<JiraActivityDto>;
  /** Opens an issue of a Jira profile in the browser. */
  openJiraIssue(account: string, key: string): Promise<void>;
  /** The tickets to offer an entry of the day: planned, used that day, periods', then recent ones. No network. */
  issueOptions(day: string): Promise<IssueOptionDto[]>;
  /**
   * Jira issues created within `weeks` (1–4) of the day with every word of `text` in their summary (a full key
   * finds that issue too), nearest to the day first. Goes to Jira with every Jira profile.
   */
  searchJiraIssues(day: string, text: string, weeks: number): Promise<IssueOptionDto[]>;
  /** The commits a git entry was made from, oldest first; none for other entries. No network. */
  worklogCommits(id: number): Promise<CommitDto[]>;
  /** What a commit changed, read from its repository. */
  commitFiles(repoId: number, hash: string): Promise<CommitFileDto[]>;
  /** A commit's patch, of one file (as commitFiles names it) or, with '', all of it. */
  commitDiff(repoId: number, hash: string, path: string): Promise<CommitDiffDto>;
  /** What you did in Confluence over the days (inclusive), with every Confluence profile. Goes to Confluence each time. */
  confluenceActivity(from: string, to: string): Promise<ConfluenceActivityDto>;
  /** Opens a page of a Confluence profile's site in the browser. */
  openConfluencePage(profile: string, pageId: string): Promise<void>;
  /** What you changed on one page on a day, version by version. Goes to Confluence (two reads per version). */
  confluenceChanges(profile: string, kind: string, pageId: string, day: string): Promise<ConfluenceChangeDto[]>;
  /** What is stored for the days (inclusive), per source page. No network. */
  sourceCounts(from: string, to: string): Promise<SourceCountsDto>;

  /** Saves an edit. A change to start, duration, issue, description or status locks the entry. */
  updateWorklog(w: WorklogDto): Promise<WorklogDto>;
  /** A duration as typed: "1h 30m", "1.5h", "1,5h", "90m", "1:30" or minutes. Rejects anything else. */
  setDuration(id: number, text: string): Promise<WorklogDto>;
  /** Drag between days. Exported entries stay put. */
  moveWorklogs(ids: number[], toDay: string): Promise<void>;
  addWorklog(day: string): Promise<WorklogDto>;
  /** Entries sent through the Tempo API can't be deleted (the backend refuses). */
  deleteWorklog(id: number): Promise<void>;
  /** All or none. */
  deleteWorklogs(ids: number[]): Promise<void>;
  setStatus(ids: number[], status: Status): Promise<void>;
  setIssue(ids: number[], issueKey: string): Promise<void>;
  /** The same comment for Tempo on several entries (they lock). */
  setComment(ids: number[], comment: string): Promise<void>;
  /** Each of the day's drafts without a comment gets its own description as the comment. */
  useDescriptionsAsComments(day: string): Promise<void>;
  /** By ticket: ticked = typed into Tempo by hand (exported, locked); unticked goes back to reviewed. */
  setEntered(ids: number[], entered: boolean): Promise<void>;

  saveSettings(s: SettingsDto): Promise<SettingsDto>;

  // Settings edits by intent: the backend makes the ids, normalises the input and keeps the rules.
  // Each resolves to the saved settings.
  /** A period over the month, to be named and given its issue. */
  addPeriod(month: string): Promise<SettingsDto>;
  /** The issue is normalised; an end before the start moves to the start. */
  updatePeriod(p: ProjectPeriod): Promise<SettingsDto>;
  removePeriod(id: string): Promise<SettingsDto>;
  addMeetingRule(contains: string, issueKey: string): Promise<SettingsDto>;
  removeMeetingRule(id: string): Promise<SettingsDto>;
  addAbsenceRule(contains: string, issueKey: string): Promise<SettingsDto>;
  removeAbsenceRule(id: string): Promise<SettingsDto>;
  /** An empty issue removes the rule. */
  setChatRule(conversationId: string, name: string, issueKey: string): Promise<SettingsDto>;
  addWorkspaceRoot(path: string): Promise<SettingsDto>;
  removeWorkspaceRoot(path: string): Promise<SettingsDto>;
  /** Where the SQLite file lives. */
  databasePath(): Promise<string>;
  /** Native folder picker; null when cancelled. */
  chooseFolder(): Promise<string | null>;
  /** Shows a path in the system file manager. */
  revealPath(path: string): Promise<void>;

  repos(month: string): Promise<RepoDto[]>;
  updateRepo(r: RepoDto): Promise<void>;
  /** Off the list only; the repository on disk is not touched. */
  removeRepo(id: number): Promise<void>;
  /** Finds git repositories under the workspace folders. */
  scanWorkspace(): Promise<void>;
  projects(): Promise<ProjectDto[]>;
  saveProject(p: ProjectDto): Promise<void>;
  removeProject(key: string): Promise<void>;

  sessions(month: string): Promise<{ days: SessionDayDto[] }>;
  clearSessions(from: string, to: string): Promise<void>;

  meetings(from: string, to: string): Promise<MeetingsDto>;
  setMeetingSkipped(id: string, skipped: boolean): Promise<void>;
  calendarStatus(): Promise<CalendarStatus>;
  /** OAuth loopback + PKCE in the system browser. */
  connectGoogle(): Promise<void>;
  disconnectGoogle(): Promise<void>;

  chats(from: string, to: string): Promise<ChatsDto>;
  /** Read on demand; message text is never logged. */
  chatMessages(id: string): Promise<ChatMessageDto[]>;
  logChat(id: string, issueKey: string): Promise<void>;
  clearChats(from: string, to: string): Promise<void>;
  /** Base64 zlib JSON or plain JSON. Resolves to how many messages came in; rejects with the reason. */
  importChats(text: string): Promise<number>;

  /** The last sign-in check of a profile's window. null: not checked yet. */
  signedIn(profileId: WebSource): Promise<boolean | null>;
  openSite(source: WebSource): Promise<void>;
  signOut(source: WebSource): Promise<void>;

  // Tokens never come back to the frontend: it only learns whether one is saved.
  /** A Jira profile's Jira or Tempo token is saved (false while locked). */
  hasToken(accountId: string, kind: TokenKind): Promise<boolean>;
  /** An empty value deletes it. Rejects while locked. */
  setToken(accountId: string, kind: TokenKind, value: string): Promise<void>;
  hasGoogleClientSecret(): Promise<boolean>;
  /** An empty value deletes it. */
  setGoogleClientSecret(value: string): Promise<void>;

  // Profiles by intent; tokens move and go with them in the backend.
  /** A Tempo profile needs a Jira profile without one: `needsJira` then, and nothing added. */
  addProfile(kind: ProfileKind): Promise<ProfileEdit>;
  /** A Jira profile takes its tokens and its Tempo profile along; a Tempo profile its token. Needs the keyring. */
  removeProfile(id: string): Promise<SettingsDto>;
  /** A Tempo profile moved to another Jira profile takes its token along. */
  updateProfile(p: Profile): Promise<SettingsDto>;
  updateJiraAccount(a: JiraAccount): Promise<SettingsDto>;
  setDefaultJiraAccount(id: string): Promise<SettingsDto>;
  checkJira(accountId: string): Promise<CheckLine[]>;
  checkTempo(accountId: string): Promise<CheckLine[]>;
  /** Signs in to an Email profile's server and finds its Sent folder. */
  checkEmail(profileId: string): Promise<CheckLine[]>;
  /** The sent mail Collect stored for the days (inclusive), by day and thread. No network. */
  sentEmails(from: string, to: string): Promise<EmailsDto>;
  /** Tests a Confluence profile's site, sign-in and permissions. */
  checkConfluence(profileId: string): Promise<CheckLine[]>;
  /** Reads the site's project keys into the account. */
  readProjects(accountId: string): Promise<string[]>;

  /** Markdown checklist for Claude in Chrome. */
  exportText(ids: number[]): Promise<string>;
  /** Save dialog, then writes the file. Resolves to the path, or null when cancelled. */
  exportFile(ids: number[], format: ExportFormat): Promise<string | null>;
  /**
   * Sends one day's reviewed entries to Tempo (or Jira), by hand. Refused while the day has drafts or hasn't come
   * yet; what Tempo already has for the day is left out. Runs as a job: ends with `collect:done` and its message.
   */
  pushDayToTempo(day: string): Promise<void>;
  /** What pushRangeToTempo would send for the days (inclusive), for the question asked first. No network. */
  pushPreview(from: string, to: string): Promise<PushPreviewDto>;
  /**
   * Sends the reviewed entries of a week, a month or any days (inclusive) to Tempo, like pushDayToTempo for each
   * day: days with drafts and days that haven't come are left out. Runs as a job, ending with `collect:done`.
   */
  pushRangeToTempo(from: string, to: string): Promise<void>;

  /**
   * Emits `collect:progress` while it runs and `collect:done` when it ends (finished or cancelled). Unless forced,
   * it leaves out the weeks Tempo has in full (`collect:done` counts them as settled).
   */
  collect(from: string, to: string, force?: boolean): Promise<void>;
  /** Deletes the local commits, meetings and chat messages of the settled days in the range; worklogs stay. */
  cleanUpSettled(from: string, to: string): Promise<CleanupDto>;
  rebuild(from: string, to: string): Promise<void>;
  fetchHrmaster(from: string, to: string): Promise<void>;
  fetchGoogleChat(from: string, to: string): Promise<void>;
  /** Google Calendar from calendar.google.com (signed in through Google Chat; no API keys), plus the ICS feeds. */
  fetchGoogleCalendar(from: string, to: string): Promise<void>;
  cancel(): Promise<void>;
  onEvent<T>(name: string, cb: (data: T) => void): () => void;
  /** This session's background activity (git, HTTP, signed-in pages, job progress), oldest first; then `activity:line`. */
  activity(): Promise<ActivityLine[]>;
}

export const API = new InjectionToken<Api>('Api');
