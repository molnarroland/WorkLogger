import { at } from '../../shared/worklogs';
import type {
  ChatsDto,
  ChatMessageDto,
  DayDto,
  MeetingsDto,
  MonthDto,
  ProjectDto,
  RepoDto,
  SessionDayDto,
  SettingsDto,
  SetupStepDto,
  WeekDto,
  WorklogDto,
} from '../api';
import sample from './sample.json';

/**
 * Shared by FakeApi, stories and specs. The months, entries and days are what the Go backend sends for its sample
 * data (testsupport.SampleMonth, written by `task fixtures`): coding + chats + standup every workday, leave on the
 * 9th, an entry without a ticket on the 27th, the 29th–30th still empty, today the 30th.
 */

const data = sample as unknown as {
  today: string;
  months: Record<string, MonthDto>;
  emptyMonths: Record<string, MonthDto>;
  weeks: Record<string, WeekDto>;
  emptyWeeks: Record<string, WeekDto>;
  sessions: Record<string, { days: SessionDayDto[] }>;
  setup: Record<SampleSetup, SetupStepDto[]>;
  meetings: Record<string, MeetingsDto>;
  chats: Record<string, ChatsDto>;
  entries: Record<'draftGitEntry' | 'lockedEntry' | 'exportedEntry' | 'needsTicketEntry' | 'leaveEntry', WorklogDto>;
  days: Record<'workday' | 'leaveDay' | 'restDay', DayDto>;
};

/**
 * Setup steps as the backend computes them: `configured` for defaultSettings, `fresh` for freshSettings, and the
 * setup-checklist and welcome-sheet stories' own settings.
 */
export type SampleSetup = 'configured' | 'fresh' | 'checklist' | 'welcome';
export const sampleSetup = (name: SampleSetup): SetupStepDto[] => structuredClone(data.setup[name]);

/** The day the sample data is computed for. */
export const SAMPLE_TODAY = data.today;

/** The sample week holding `day` (September to November 2026), or null. Its prev/next step a week from its Monday. */
export function sampleWeek(day: string, empty = false): WeekDto | null {
  const w = Object.values(empty ? data.emptyWeeks : data.weeks).find((x) => x.days.some((d) => d.day === day));
  return w ? structuredClone(w) : null;
}

/** A month of sample data (September to November 2026); other months come back blank. */
export function sampleMonth(month: string, empty = false): MonthDto {
  const m = (empty ? data.emptyMonths : data.months)[month];
  if (m) return structuredClone(m);
  return {
    month,
    today: data.today,
    currentMonth: data.today.slice(0, 7),
    from: `${month}-01`,
    to: `${month}-01`,
    prev: month,
    next: month,
    openDay: `${month}-01`,
    anchorDay: `${month}-01`,
    weeks: [],
    days: [],
    worklogs: [],
    inTempo: [],
    settled: [],
    summary: {
      logged: 0,
      expected: 0,
      short: 0,
      behind: 0,
      behindDays: [],
      inTempo: 0,
      inTempoDays: 0,
      toLog: 0,
      draftDays: [],
      missing: [],
      missingDays: [],
      missingSeconds: 0,
      parts: [],
      barParts: [],
    },
    todos: [],
    attention: [],
    drafts: [],
    queue: [],
    tickets: [],
    tempoProgress: { entered: 0, total: 0 },
    needsTicket: 0,
    reports: { day: [], week: [], month: [] },
    exports: [],
  };
}

/** The default settings, plus a filled-in setup. */
export const defaultSettings = (): SettingsDto => ({
  modules: ['git', 'calendar', 'workTimes', 'chats'],
  tempoByHand: true,
  welcomed: true,
  workspaceRoot: '~/Workspaces',
  workspaceRoots: ['~/Workspaces'],
  authorEmails: ['me@example.com'],
  excludedFolders: ['archive'],
  ignoredPathPatterns: [
    'package-lock.json',
    'composer.lock',
    'yarn.lock',
    'pnpm-lock.yaml',
    '.min.js',
    '/dist/',
    '/vendor/',
    'node_modules/',
  ],
  minCommitMinutes: 10,
  maxCommitMinutes: 240,
  maxCommitGapMinutes: 180,
  roundingMode: 'fixedHours',
  fixedDailyHours: 8,
  granularityMinutes: 15,
  dayRoundUpMinutes: 15,
  defaultDayStart: '09:00',
  defaultDayEnd: '17:30',
  includeWeekends: false,
  language: 'system',
  appearance: 'system',
  accentColor: 'system',
  skin: 'system',
  transparency: 'system',
  contrast: 'system',
  motion: 'system',
  textScale: 0,
  chatGapMinutes: 10,
  chatRules: [{ conversationId: 'space/orb', name: 'Trip planner', issueKey: 'ORB-214' }],
  chatEmail: 'me@example.com',
  hrmasterURL: 'https://example.hrmaster.hu',
  profiles: [
    { id: 'googleChat', kind: 'googleChat', name: '', email: 'me@example.com', url: '', jiraAccountId: '' },
    {
      id: 'hrmaster',
      kind: 'hrmaster',
      name: 'Example Travel',
      email: '',
      url: 'https://example.hrmaster.hu',
      jiraAccountId: '',
    },
    { id: 'tempo-j1', kind: 'tempo', name: '', email: '', url: 'https://api.tempo.io/4', jiraAccountId: 'j1' },
  ],
  absenceIssueKey: 'LEAVE-1',
  absenceRules: [{ id: 'a1', contains: 'Betegszabadság', issueKey: 'LEAVE-2' }],
  periods: [
    {
      id: 'p1',
      title: 'Map provider migration',
      issueKey: 'ORB-300',
      from: '2026-10-19',
      to: '2026-10-23',
      comment: 'Map provider migration',
    },
  ],
  googleClientId: '',
  googleCalendarId: 'primary',
  icsUrls: [],
  defaultMeetingIssue: 'TEAM-12',
  meetingRules: [{ id: 'm1', contains: 'Sprint planning', issueKey: 'TEAM-7' }],
  skipMeetingKeywords: ['Lunch', 'Focus time'],
  jiraAccounts: [
    {
      id: 'j1',
      name: 'Example Travel',
      baseURL: 'https://example-travel.atlassian.net',
      email: 'me@example.com',
      accountId: '5b10ac8d82e05b22cc7d4ef5',
      provider: 'tempo',
      tempoBaseURL: 'https://api.tempo.io/4',
      projectKeys: ['ORB', 'NOVA', 'TEAM', 'LEAVE'],
    },
  ],
  defaultJiraAccountId: 'j1',
});

/** A fresh install: nothing set up, welcome not seen. */
export const freshSettings = (): SettingsDto => ({
  ...defaultSettings(),
  welcomed: false,
  workspaceRoot: '',
  workspaceRoots: [],
  authorEmails: [],
  excludedFolders: [],
  chatRules: [],
  hrmasterURL: '',
  chatEmail: '',
  profiles: [],
  absenceIssueKey: '',
  absenceRules: [],
  periods: [],
  defaultMeetingIssue: '',
  meetingRules: [],
  skipMeetingKeywords: [],
  jiraAccounts: [],
  defaultJiraAccountId: null,
});

export const sampleRepos = (): RepoDto[] => [
  {
    id: 1,
    path: '~/Workspaces/trip-planner',
    name: 'trip-planner',
    issueKey: null,
    projectKey: 'ORB',
    enabled: true,
    commits: 42,
    days: { '2026-10-01': 9, '2026-10-05': 12, '2026-10-06': 6, '2026-10-08': 8, '2026-10-12': 7 },
  },
  {
    id: 2,
    path: '~/Workspaces/web-dashboard',
    name: 'web-dashboard',
    issueKey: 'NOVA-58',
    projectKey: 'NOVA',
    enabled: true,
    commits: 17,
    days: { '2026-10-06': 5, '2026-10-13': 4, '2026-10-14': 8 },
  },
  {
    id: 3,
    path: '~/Workspaces/old-tools/report-scripts',
    name: 'report-scripts',
    issueKey: null,
    projectKey: null,
    enabled: false,
    commits: 0,
    days: {},
  },
];

export const sampleProjects = (): ProjectDto[] => [
  { key: 'ORB', defaultIssue: 'ORB-1' },
  { key: 'NOVA', defaultIssue: null },
];

/** HRmaster-like sessions as the backend sends them (08:30–17:00 with a lunch break, leave on the 9th). */
export function sampleSessions(month: string): { days: SessionDayDto[] } {
  return structuredClone(data.sessions[month] ?? { days: [] });
}

/** The month's meetings as the backend sends them, with the story's meeting rules applied. */
export function sampleMeetings(month: string): MeetingsDto {
  return structuredClone(data.meetings[month] ?? { meetings: [], suggestion: null });
}

/** Two conversations on the 28th, as the backend sends them. */
export function sampleChats(month: string): ChatsDto {
  return structuredClone(data.chats[month] ?? { days: [], active: 0 });
}

export function sampleMessages(chatId: string): ChatMessageDto[] {
  const day = chatId.slice(0, 10);
  if (chatId.endsWith('dm/mira'))
    return [
      {
        time: at(day, '14:40'),
        sender: 'mira@example.com',
        me: false,
        text: 'Do you have a minute for the trip export?',
        files: [],
      },
      { time: at(day, '14:41'), sender: 'me@example.com', me: true, text: 'Sure, what breaks?', files: [] },
      { time: at(day, '14:45'), sender: 'mira@example.com', me: false, text: '', files: ['export-2026-10.csv'] },
      {
        time: at(day, '14:50'),
        sender: 'me@example.com',
        me: true,
        text: 'Time zones again. Fix is on ORB-214, shipping tomorrow.',
        files: [],
      },
      { time: at(day, '14:56'), sender: 'mira@example.com', me: false, text: 'Thanks!', files: [] },
      { time: at(day, '14:58'), sender: 'me@example.com', me: true, text: '👍', files: [] },
    ];
  return [
    { time: at(day, '10:02'), sender: 'me@example.com', me: true, text: 'Deploying the fix to test', files: [] },
    { time: at(day, '10:05'), sender: 'mira@example.com', me: false, text: 'Thanks!', files: ['log.txt'] },
    { time: at(day, '10:12'), sender: 'me@example.com', me: true, text: 'Done, test is green.', files: [] },
  ];
}

export const fixtures = {
  settings: defaultSettings(),
  gitOnly: { ...defaultSettings(), modules: ['git'] } satisfies SettingsDto,
  /** A collect's progress, as the backend sends it (codes from internal/core/messages.go). */
  collectSteps: [
    {
      source: 'git',
      message: { code: 'progress.git', params: { n: 2 }, text: 'Reading git history of 2 repositories…' },
    },
    { source: 'calendar', message: { code: 'progress.calendar', text: 'Reading meetings from your calendars…' } },
    { source: 'drafts', message: { code: 'progress.drafts', text: 'Building drafts…' } },
  ],

  // Single entries and days for component stories.
  ...structuredClone(data.entries),
  ...structuredClone(data.days),
};

export const october = () => sampleMonth('2026-10');
