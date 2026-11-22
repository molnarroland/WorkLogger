import { at, hm } from '../shared/worklogs';
import type {
  ActivityLine,
  CleanupDto,
  CollectDone,
  ChatsDto,
  TokenKind,
  ProfileKind,
  ProfileEdit,
  Profile,
  JiraAccount,
  Api,
  ChatDayDto,
  CheckLine,
  CollectProgress,
  CommitDto,
  CommitDiffDto,
  CommitFileDto,
  IssueOptionDto,
  PushPreviewDto,
  ExportFormat,
  MonthDto,
  JiraActivityDto,
  ConfluenceActivityDto,
  ConfluenceChangeDto,
  EmailsDto,
  ConfluenceDayDto,
  SourceCountsDto,
  JiraDayDto,
  JiraTicketDto,
  PlanItemDto,
  PlansDto,
  ProjectDto,
  ProjectPeriod,
  RepoDto,
  SettingsDto,
  Status,
  WebSource,
  WorklogDto,
} from './api';
import type { LanguageChoice } from '../i18n/t.pipe';
import { EVENTS } from './api';
import {
  SAMPLE_MONTHS,
  SAMPLE_TODAY,
  sampleWeek,
  sampleSetup,
  defaultSettings,
  fixtures,
  sampleChats,
  sampleMeetings,
  sampleMessages,
  sampleMonth,
  sampleProjects,
  sampleRepos,
  sampleSessions,
} from './testing/fixtures';

export interface FakeApiOptions {
  locked?: boolean;
  /** How unlock answers: false simulates the user cancelling the keyring prompt. */
  unlockSucceeds?: boolean;
  settings?: SettingsDto;
  /** The settings' UI language; stories pin it, since the shell applies the settings' language once they load. */
  language?: LanguageChoice;
  /** Months to serve instead of the generated sample, keyed "YYYY-MM". */
  months?: Record<string, MonthDto>;
  /** Empty lists everywhere: a fresh install. */
  empty?: boolean;
  /** Sites signed in; default both. */
  signedIn?: Partial<Record<WebSource, boolean | null>>;
  /** Jira check fails with a missing scope. */
  jiraCheckFails?: boolean;
  /** Delay between collect steps. 0 in tests. */
  stepMs?: number;
}

/**
 * In-memory Api for Storybook, specs, e2e and `ng serve` in a plain browser. Months come from the Go-generated
 * sample (fixtures.ts). Mutations persist per instance but are naive: an edited entry's own flags follow its
 * fields (as WorklogDto documents them), but what the backend derives across entries (day states, totals, to-dos,
 * tickets) stays as generated.
 */
export class FakeApi implements Api {
  private locked: boolean;
  private cancelled = false;
  private settingsValue: SettingsDto;
  private readonly months = new Map<string, MonthDto>();
  private readonly listeners = new Map<string, Set<(data: unknown) => void>>();
  private repoList: RepoDto[];
  private projectList: ProjectDto[];
  private readonly skipped = new Set<string>();
  private readonly cleared = { sessions: new Set<string>(), chats: new Set<string>() };
  private readonly chatIssues = new Map<string, string>();
  /** Saved tokens by "kind|account"; the Google client secret under "google". */
  private readonly secrets = new Map<string, string>([['jira|j1', 'atl-xxxxxxxx']]);
  private readonly sites: Record<string, boolean | null>;
  private googleConnected = false;
  private importedChats: ChatDayDto[] = [];

  constructor(private readonly opts: FakeApiOptions = {}) {
    this.locked = opts.locked ?? false;
    this.settingsValue = structuredClone(opts.settings ?? defaultSettings());
    if (opts.language) this.settingsValue.language = opts.language;
    for (const [k, v] of Object.entries(opts.months ?? {})) this.months.set(k, structuredClone(v));
    this.repoList = opts.empty ? [] : sampleRepos();
    this.projectList = opts.empty ? [] : sampleProjects();
    this.sites = { hrmaster: true, googleChat: true, ...opts.signedIn };
  }

  // ── Lock, settings ─────────────────────────────────────────────────────────

  async isLocked() {
    return this.locked;
  }

  async unlock() {
    if (this.opts.unlockSucceeds ?? true) this.locked = false;
    return !this.locked;
  }

  async settings() {
    return structuredClone(this.settingsValue);
  }

  /** Naive: the generated steps for a fresh install while nothing is set up, else for the configured sample. */
  async setupSteps() {
    const s = this.settingsValue;
    return sampleSetup(s.profiles.length || s.workspaceRoots.length ? 'configured' : 'fresh');
  }

  async saveSettings(s: SettingsDto) {
    this.settingsValue = structuredClone(s);
    return structuredClone(s);
  }

  async addPeriod(month: string) {
    const [y, m] = month.split('-').map(Number);
    const last = new Date(y, m, 0).getDate();
    const p = {
      id: crypto.randomUUID(),
      title: '',
      issueKey: '',
      from: `${month}-01`,
      to: `${month}-${last}`,
      comment: '',
    };
    return this.change((s) => s.periods.push(p));
  }

  async updatePeriod(p: ProjectPeriod) {
    const to = p.to < p.from ? p.from : p.to;
    return this.change((s) => (s.periods = s.periods.map((x) => (x.id === p.id ? { ...p, to } : x))));
  }

  async removePeriod(id: string) {
    return this.change((s) => (s.periods = s.periods.filter((x) => x.id !== id)));
  }

  async addMeetingRule(contains: string, issueKey: string) {
    return this.change((s) => s.meetingRules.push({ id: crypto.randomUUID(), contains, issueKey }));
  }

  async removeMeetingRule(id: string) {
    return this.change((s) => (s.meetingRules = s.meetingRules.filter((r) => r.id !== id)));
  }

  async addAbsenceRule(contains: string, issueKey: string) {
    return this.change((s) => s.absenceRules.push({ id: crypto.randomUUID(), contains, issueKey }));
  }

  async removeAbsenceRule(id: string) {
    return this.change((s) => (s.absenceRules = s.absenceRules.filter((r) => r.id !== id)));
  }

  async setChatRule(conversationId: string, name: string, issueKey: string) {
    return this.change((s) => {
      s.chatRules = s.chatRules.filter((r) => r.conversationId !== conversationId);
      if (issueKey) s.chatRules.push({ conversationId, name, issueKey });
    });
  }

  async addWorkspaceRoot(path: string) {
    return this.change((s) => {
      if (!s.workspaceRoots.includes(path)) s.workspaceRoots.push(path);
    });
  }

  async removeWorkspaceRoot(path: string) {
    return this.change((s) => (s.workspaceRoots = s.workspaceRoots.filter((r) => r !== path)));
  }

  /** Naive: applies the edit as given (the backend also normalises and validates). */
  private change(edit: (s: SettingsDto) => unknown) {
    edit(this.settingsValue);
    return Promise.resolve(structuredClone(this.settingsValue));
  }

  async databasePath() {
    return '~/.config/WorkLogger/worklogger.sqlite';
  }

  async chooseFolder() {
    return '~/Workspaces';
  }

  async revealPath() {
    // Nothing to reveal outside the desktop app.
  }

  // ── Month and worklogs ─────────────────────────────────────────────────────

  /** The sample week; outside the sample, the current one. Naive: edits made here don't show in it. */
  async week(day: string) {
    return sampleWeek(day || SAMPLE_TODAY, this.opts.empty) ?? sampleWeek(SAMPLE_TODAY, this.opts.empty)!;
  }

  /** The sample has no Tempo to read back. */
  async refreshTempo() {
    return false;
  }

  /** An empty month is the sample's current one, as the backend does with its clock. */
  async month(month: string) {
    const m = this.get(month || SAMPLE_TODAY.slice(0, 7));
    m.worklogs.forEach(entryFlags);
    const planned = samplePlanned(m.worklogs);
    for (const w of m.worklogs) w.planIssue = w.needsIssue ? (planned.get(w.day) ?? null) : null;
    for (const t of m.tickets) for (const d of t.days) d.planned = d.key && planned.get(d.day) === d.key ? 4 * 3600 : 0;
    return structuredClone(m);
  }

  /** Counts from the sample: what the pages would list for the days. */
  async sourceCounts(from: string, to: string): Promise<SourceCountsDto> {
    if (this.opts.empty)
      return { repositories: 0, meetings: 0, sessions: 0, chats: 0, jira: 0, confluence: 0, email: 0 };
    const logs = (await this.month(from.slice(0, 7))).worklogs.filter((w) => w.day >= from && w.day <= to);
    const count = (source: string) => logs.filter((w) => w.source === source).length;
    return {
      repositories: new Set(logs.flatMap((w) => (w.focus?.pane === 'repositories' ? [w.focus.id] : []))).size,
      meetings: count('meeting'),
      sessions: new Set(logs.map((w) => w.day)).size,
      chats: count('chat'),
      jira: count('jira'),
      confluence: count('confluence'),
      email: count('email'),
    };
  }

  /** Jira activity made up from the sample: per day, its first ticket moved and commented; untagged entries suggested. */
  async jiraActivity(from: string, to: string): Promise<JiraActivityDto> {
    if (this.opts.empty) return { days: [], entries: 0, tickets: 0, accounts: 0 };
    const m = await this.month(from.slice(0, 7));
    const days: JiraDayDto[] = [];
    const dayKeys = [...new Set(m.worklogs.map((w) => w.day))].filter((d) => d >= from && d <= to);
    for (const [i, day] of dayKeys.sort().reverse().entries()) {
      const w = m.worklogs.find((x) => x.day === day && x.issueKey);
      if (!w?.issueKey) continue;
      const ticket: JiraTicketDto = {
        account: 'acme',
        accountTitle: '',
        issueKey: w.issueKey,
        summary: w.description || w.issueKey,
        first: '09:05',
        last: '16:20',
        created: i === dayKeys.length - 1,
        statuses: [{ time: '09:05', field: 'status', from: 'To Do', to: 'In Progress' }],
        changes: i % 2 ? [{ time: '09:06', field: 'assignee', from: '', to: 'Me' }] : [],
        comments: [{ time: '16:20', text: 'Pushed the fix, ready for review.' }],
        attachments: i % 3 === 0 ? [{ time: '16:21', text: 'screenshot.png' }] : [],
        suggested: m.worklogs
          .filter((x) => x.day === day && x.needsIssue)
          .map((x) => ({
            id: x.id,
            source: x.source,
            description: x.tempoText,
            seconds: x.duration,
            status: x.status,
            focus: x.focus,
          })),
      };
      days.push({ day, tickets: [ticket] });
    }
    return { days, entries: days.length * 2, tickets: days.length, accounts: 1 };
  }

  async openJiraIssue() {
    // The fake has no browser to open.
  }

  /** The tickets of the sample month: the day's first, then the rest by when they were last used. */
  async issueOptions(day: string): Promise<IssueOptionDto[]> {
    const m = await this.month(day.slice(0, 7));
    const byKey = new Map<string, IssueOptionDto>();
    for (const w of m.worklogs) {
      if (!w.issueKey) continue;
      const o = byKey.get(w.issueKey) ?? { ...issueOption(w.issueKey, ''), origin: 'recent' as const };
      o.uses++;
      if (w.day > o.day) o.day = w.day;
      if (w.day === day) o.origin = 'day';
      byKey.set(w.issueKey, o);
    }
    return [...byKey.values()].sort(
      (a, b) => Number(b.origin === 'day') - Number(a.origin === 'day') || b.day.localeCompare(a.day),
    );
  }

  /** Invented issues created around the day, filtered by the words of the text. */
  async searchJiraIssues(day: string, text: string, weeks: number): Promise<IssueOptionDto[]> {
    if (this.locked) throw new Error('Unlock first: tokens are kept in the keyring.');
    const base = new Date(`${day}T12:00:00`);
    const titles = [
      'Login page times out on slow networks',
      'Export the monthly report as CSV',
      'Rounding of commit estimates',
      'Payment webhook retries',
      'Dark mode for the settings page',
      'Upgrade the build pipeline',
    ];
    const descriptions = [
      'On 3G the sign-in request takes over 30 seconds and the page gives up. Raise the timeout and show progress.',
      'Finance wants the month as a CSV with one row per day and ticket, hours with two decimals.',
      '',
      'Stripe retries a failed webhook for three days; make the handler idempotent so a retry books nothing twice.',
      'Follow the system appearance and remember an override per user.',
      'Move CI to the new runners and cache the Go modules between jobs.',
    ];
    const words = text
      .toLocaleLowerCase()
      .split(/[^\p{L}\p{N}]+/u)
      .filter(Boolean);
    return titles
      .map((summary, i) => {
        const created = new Date(base);
        created.setDate(base.getDate() + (i % 2 ? 1 : -1) * Math.round(((i + 1) * 7 * weeks) / titles.length));
        return {
          ...issueOption(`WL-${120 + i}`, summary),
          origin: 'jira' as const,
          day: created.toISOString().slice(0, 10),
          status: i % 3 ? 'In Progress' : 'To Do',
          description: descriptions[i],
          account: 'j1',
        };
      })
      .filter((o) =>
        words.every((w) =>
          o.summary
            .toLocaleLowerCase()
            .split(/\s+/)
            .some((x) => x.startsWith(w)),
        ),
      );
  }

  /** Two made-up commits for a git entry. */
  async worklogCommits(id: number): Promise<CommitDto[]> {
    for (const month of SAMPLE_MONTHS) this.get(month); // every sample month, loaded once
    const w = this.find(id);
    if (!w || w.source !== 'git') return [];
    const repo = this.repoList[id % Math.max(this.repoList.length, 1)];
    const key = w.issueKey ?? '';
    return [
      { minutes: 0, subject: `${key} Fix rounding of the last entry`.trim(), files: 3, ins: 42, del: 7 },
      { minutes: 95, subject: `${key} Cover daylight-saving days`.trim(), files: 1, ins: 18, del: 2 },
    ].map((c, i) => ({
      repoId: repo?.id ?? 1,
      repo: repo?.name ?? 'billing',
      hash: `${(id * 7919 + i).toString(16).padStart(8, '0')}c0ffee${i}`,
      at: new Date(new Date(w.start).getTime() + c.minutes * 60_000).toISOString(),
      author: 'You',
      subject: c.subject,
      body: i === 0 ? 'The remainder went to the first entry instead of the largest one.' : '',
      ref: 'refs/heads/main',
      files: c.files,
      insertions: c.ins,
      deletions: c.del,
    }));
  }

  async commitFiles(_repoId: number, hash: string): Promise<CommitFileDto[]> {
    return hash.endsWith('0')
      ? [
          { path: 'internal/core/builder.go', added: 30, deleted: 6 },
          { path: 'internal/core/builder_test.go', added: 11, deleted: 1 },
          { path: 'docs/{notes.md => ARCHITECTURE.md}', added: 1, deleted: 0 },
        ]
      : [{ path: 'internal/core/days_test.go', added: 18, deleted: 2 }];
  }

  async commitDiff(_repoId: number, _hash: string, path: string): Promise<CommitDiffDto> {
    const file = path || 'internal/core/builder.go';
    return {
      text: [
        `diff --git a/${file} b/${file}`,
        'index 3f2a1c0..9b8e7d1 100644',
        `--- a/${file}`,
        `+++ b/${file}`,
        '@@ -120,7 +120,9 @@ func share(total int, parts []int) []int {',
        ' \tfor i := range parts {',
        '-\t\tout[0] += rest',
        '+\t\t// The remainder goes to the largest part.',
        '+\t\tout[largest(parts)] += rest',
        ' \t}',
        ' \treturn out',
        ' }',
      ].join('\n'),
      truncated: false,
    };
  }

  /** Two pages a day from the sample: a spec named after the day's ticket (with its draft), and a roadmap. */
  async confluenceActivity(from: string, to: string): Promise<ConfluenceActivityDto> {
    if (this.opts.empty) return { days: [], entries: 0, pages: 0, profiles: 0 };
    const m = await this.month(from.slice(0, 7));
    const entry = (x: (typeof m.worklogs)[number]) => ({
      id: x.id,
      source: x.source,
      description: x.tempoText,
      seconds: x.duration,
      status: x.status,
      focus: x.focus,
    });
    const days: ConfluenceDayDto[] = [];
    const dayKeys = [...new Set(m.worklogs.map((w) => w.day))].filter((d) => d >= from && d <= to);
    for (const day of dayKeys.sort().reverse()) {
      const w = m.worklogs.find((x) => x.day === day && x.issueKey);
      if (!w?.issueKey) continue;
      const base = { profile: 'c1', profileTitle: '', kind: 'page', space: 'Product', suggested: [], entry: null };
      days.push({
        day,
        pages: [
          {
            ...base,
            pageId: '2',
            title: 'Roadmap',
            first: '14:10',
            last: '14:10',
            edits: [],
            comments: [{ time: '14:10', text: 'Moved the export to Q3, see the spec.' }],
            issueKey: '',
          },
          {
            ...base,
            pageId: '1',
            title: `${w.issueKey} Spec`,
            first: '09:40',
            last: '11:15',
            edits: ['09:40', '10:30', '11:15'],
            comments: [],
            issueKey: w.issueKey,
            entry: entry(w),
            suggested: m.worklogs.filter((x) => x.day === day && x.needsIssue).map(entry),
          },
        ],
      });
    }
    return { days, entries: days.length * 4, pages: days.length * 2, profiles: 1 };
  }

  async openConfluencePage() {
    // The fake has no browser to open.
  }

  async confluenceChanges(): Promise<ConfluenceChangeDto[]> {
    return [
      {
        time: '09:40',
        number: 4,
        message: '',
        added: 2,
        removed: 0,
        addedText: ['Scope', 'Out of scope: exports'],
        removedText: [],
      },
      {
        time: '11:15',
        number: 6,
        message: 'Agreed with the team',
        added: 1,
        removed: 1,
        addedText: ['Exports move to Q3'],
        removedText: ['Exports in Q2'],
      },
    ];
  }

  /** Plans made up from the sample: each day's biggest ticket is planned 4h; Tempo has a 15m status-change log. */
  async plans(month: string): Promise<PlansDto> {
    if (this.opts.empty) return { days: [], planned: 0, read: true };
    const m = await this.month(month);
    const planned = samplePlanned(m.worklogs);
    const entry = (w: WorklogDto) => ({
      id: w.id,
      source: w.source,
      description: w.tempoText,
      seconds: w.duration,
      status: w.status,
      focus: w.focus,
    });
    const days = [...planned].map(([day, key], i) => {
      const onDay = m.worklogs.filter((w) => w.day === day && w.duration > 0);
      const entries = onDay.filter((w) => w.issueKey === key);
      const items: PlanItemDto[] = [
        {
          kind: 'plan',
          key,
          day,
          seconds: 4 * 3600,
          description: 'In progress',
          logged: entries.reduce((n, w) => n + w.duration, 0),
          entries: entries.map(entry),
        },
      ];
      if (i % 3 === 0)
        items.push({ kind: 'tempo', key, day, seconds: 900, description: 'Status change', logged: 0, entries: [] });
      const untagged = onDay.filter((w) => w.issueKey === null).map(entry);
      return { day, planned: 4 * 3600, items, untagged, suggest: untagged.length ? key : null };
    });
    return { days, planned: days.length * 4 * 3600, read: true };
  }

  async updateWorklog(w: WorklogDto) {
    const old = this.find(w.id);
    if (!old) throw new Error(`No worklog ${w.id}`);
    const changed =
      old.start !== w.start ||
      old.duration !== w.duration ||
      old.issueKey !== w.issueKey ||
      old.description !== w.description ||
      old.comment !== w.comment ||
      old.status !== w.status;
    if (w.status === 'reviewed' && old.status !== 'reviewed' && !w.comment.trim())
      throw new Error('Write the comment that goes to Tempo before marking the entry reviewed.');
    Object.assign(old, w, { locked: w.locked || changed });
    return structuredClone(old);
  }

  /** Naive: "1h 30m"-style text or bare minutes (the backend takes more forms). */
  async setDuration(id: number, text: string) {
    const t = text.toLowerCase().replaceAll(',', '.');
    const unit = [...t.matchAll(/(\d+(?:\.\d+)?)\s*([hm])/g)].reduce(
      (s, m) => s + Number(m[1]) * (m[2] === 'h' ? 3600 : 60),
      0,
    );
    const seconds = /^\s*\d+\s*$/.test(t) ? Number(t) * 60 : Math.round(unit);
    if (!seconds) throw new Error(`not a duration: "${text}"`);
    const w = this.find(id);
    if (!w) throw new Error(`No worklog ${id}`);
    return this.updateWorklog({ ...w, duration: seconds });
  }

  async moveWorklogs(ids: number[], toDay: string) {
    for (const id of ids) {
      const w = this.find(id);
      if (!w || w.status === 'exported' || w.day === toDay) continue;
      this.remove(id);
      this.get(toDay.slice(0, 7)).worklogs.push({ ...w, day: toDay, start: at(toDay, hm(w.start)), locked: true });
    }
  }

  async addWorklog(day: string) {
    const w: WorklogDto = {
      id: Math.max(0, ...[...this.months.values()].flatMap((x) => x.worklogs.map((l) => l.id))) + 1,
      day,
      start: at(day, '09:00'),
      end: at(day, '09:30'),
      duration: 1800,
      issueKey: null,
      description: '',
      comment: '',
      source: 'manual',
      estimated: 1800,
      status: 'draft',
      locked: true,
      tempoId: null,
      // The derived fields as the backend would send them for a new entry.
      needsIssue: true,
      needsComment: true,
      tempoText: '',
      inTempo: false,
      effectiveStatus: 'draft',
      canDelete: true,
      canMove: true,
      startMinute: 9 * 60,
      focus: null,
      planIssue: null,
    };
    this.get(day.slice(0, 7)).worklogs.push(w);
    return structuredClone(w);
  }

  async deleteWorklog(id: number) {
    return this.deleteWorklogs([id]);
  }

  async deleteWorklogs(ids: number[]) {
    if (this.each(ids).some((w) => !w.canDelete)) throw new Error('This entry is already in Tempo; change it there.');
    for (const id of ids) this.remove(id);
  }

  async setStatus(ids: number[], status: Status) {
    const missing = this.each(ids).filter((w) => w.status === 'draft' && !w.comment.trim()).length;
    if (status === 'reviewed' && missing)
      throw new Error(
        `${missing} of the entries have no comment for Tempo yet. Write them before marking them reviewed.`,
      );
    for (const w of this.each(ids)) if (w.status !== 'exported') Object.assign(w, { status, locked: true });
  }

  async setComment(ids: number[], comment: string) {
    for (const w of this.each(ids))
      if (w.status !== 'exported') Object.assign(w, { comment: comment.trim(), locked: true });
  }

  async useDescriptionsAsComments(day: string) {
    for (const m of this.months.values())
      for (const w of m.worklogs)
        if (w.day === day && w.needsComment && w.description.trim())
          Object.assign(w, { comment: w.description.trim(), locked: true });
  }

  async setIssue(ids: number[], issueKey: string) {
    for (const w of this.each(ids)) if (w.status !== 'exported') Object.assign(w, { issueKey, locked: true });
  }

  async setEntered(ids: number[], entered: boolean) {
    for (const w of this.each(ids))
      if (w.tempoId === null) Object.assign(w, { status: entered ? 'exported' : 'reviewed', locked: true });
  }

  // ── Repositories ───────────────────────────────────────────────────────────

  async repos() {
    return structuredClone(this.repoList);
  }

  async updateRepo(r: RepoDto) {
    this.repoList = this.repoList.map((x) => (x.id === r.id ? { ...r } : x));
  }

  async removeRepo(id: number) {
    this.repoList = this.repoList.filter((r) => r.id !== id);
  }

  async scanWorkspace() {
    if (!this.repoList.length) this.repoList = sampleRepos();
  }

  async projects() {
    return structuredClone(this.projectList);
  }

  async saveProject(p: ProjectDto) {
    this.projectList = [...this.projectList.filter((x) => x.key !== p.key), { ...p }].sort((a, b) =>
      a.key.localeCompare(b.key),
    );
  }

  async removeProject(key: string) {
    this.projectList = this.projectList.filter((p) => p.key !== key);
  }

  // ── Work sessions ──────────────────────────────────────────────────────────

  async sessions(month: string) {
    if (this.opts.empty || this.cleared.sessions.has(month)) return { days: [] };
    return sampleSessions(month);
  }

  async clearSessions(from: string) {
    this.cleared.sessions.add(from.slice(0, 7));
  }

  // ── Meetings ───────────────────────────────────────────────────────────────

  /** The sample month's meetings (one day's when from = to); "skipped" follows the edits, the rest is generated. */
  async meetings(from: string, to: string) {
    if (this.opts.empty) return { meetings: [], suggestion: null };
    const m = sampleMeetings(from.slice(0, 7));
    m.meetings = m.meetings
      .filter((x) => from !== to || x.day === from)
      .map((x) => ({ ...x, skipped: this.skipped.has(x.id) }));
    return m;
  }

  async setMeetingSkipped(id: string, skipped: boolean) {
    if (skipped) this.skipped.add(id);
    else this.skipped.delete(id);
  }

  async calendarStatus() {
    return {
      googleConnected: this.googleConnected,
      googleWeb:
        !this.googleConnected &&
        (this.settingsValue.profiles ?? []).some((p) => p.kind === 'googleChat' && this.sites[p.id] === true),
    };
  }

  async connectGoogle() {
    this.googleConnected = true;
  }

  async disconnectGoogle() {
    this.googleConnected = false;
  }

  // ── Chats ──────────────────────────────────────────────────────────────────

  /** The sample month's chats (one day's when from = to) plus imported ones; a logged ticket shows. */
  async chats(from: string, to: string): Promise<ChatsDto> {
    const month = from.slice(0, 7);
    const sample = this.opts.empty || this.cleared.chats.has(month) ? { days: [], active: 0 } : sampleChats(month);
    // Naive: each imported conversation-day is a day of its own (the backend groups them).
    const days = [
      ...sample.days,
      ...this.importedChats.map((c) => ({ day: c.day, seconds: c.activeSeconds, chats: [c] })),
    ].filter((d) => from !== to || d.day === from);
    for (const d of days) d.chats = d.chats.map((c) => ({ ...c, issueKey: this.chatIssues.get(c.id) ?? c.issueKey }));
    return { days, active: sample.active };
  }

  async chatMessages(id: string) {
    return sampleMessages(id);
  }

  async logChat(id: string, issueKey: string) {
    this.chatIssues.set(id, issueKey);
  }

  async clearChats(from: string) {
    this.cleared.chats.add(from.slice(0, 7));
    this.importedChats = [];
  }

  async importChats(text: string) {
    let data: { conversations?: { id: string; name: string; type?: string; messages: { time: string }[] }[] };
    try {
      data = JSON.parse(text.trim());
    } catch {
      throw new Error('This is not a chat export: expected JSON or base64 text.');
    }
    const convs = data.conversations ?? [];
    for (const c of convs) {
      const times = c.messages.map((m) => m.time).sort();
      if (!times.length) continue;
      this.importedChats.push({
        id: `${times[0].slice(0, 10)}|${c.id}`,
        day: times[0].slice(0, 10),
        conversationId: c.id,
        name: c.name,
        type: c.type === 'dm' ? 'dm' : 'space',
        count: c.messages.length,
        activeSeconds: 60,
        first: times[0],
        last: times.at(-1)!,
        issueKey: null,
        exported: false,
        ruleIssue: null,
      });
    }
    return convs.reduce((n, c) => n + c.messages.length, 0);
  }

  // ── Signed-in sites, secrets, Jira ─────────────────────────────────────────

  async signedIn(source: WebSource) {
    return this.sites[source] ?? null;
  }

  async openSite(source: WebSource) {
    this.sites[source] = true;
  }

  async signOut(source: WebSource) {
    this.sites[source] = false;
  }

  async hasToken(accountId: string, kind: TokenKind) {
    return this.secrets.has(`${kind}|${accountId}`);
  }

  async setToken(accountId: string, kind: TokenKind, value: string) {
    this.setSecret(`${kind}|${accountId}`, value);
  }

  async hasGoogleClientSecret() {
    return this.secrets.has('google');
  }

  async setGoogleClientSecret(value: string) {
    this.setSecret('google', value);
  }

  private setSecret(key: string, value: string) {
    if (value.trim()) this.secrets.set(key, value.trim());
    else this.secrets.delete(key);
  }

  /** Naive versions of the backend's profile edits (no keyring, no validation beyond the Tempo link). */
  async addProfile(kind: ProfileKind): Promise<ProfileEdit> {
    const s = this.settingsValue;
    const id = crypto.randomUUID();
    if (kind === 'jira') {
      s.jiraAccounts.push({
        id,
        name: '',
        baseURL: '',
        email: '',
        accountId: '',
        provider: 'jira',
        tempoBaseURL: 'https://api.tempo.io/4',
        projectKeys: [],
      });
      s.defaultJiraAccountId ??= id;
      return { settings: structuredClone(s), id, needsJira: false };
    }
    const linked = new Set(s.profiles.filter((p) => p.kind === 'tempo').map((p) => p.jiraAccountId));
    const free = s.jiraAccounts.find((a) => !linked.has(a.id));
    if (kind === 'tempo' && !free) return { settings: structuredClone(s), id: '', needsJira: true };
    s.profiles.push({
      id,
      kind,
      name: '',
      email: kind === 'googleChat' ? (s.authorEmails[0] ?? '') : '',
      url: kind === 'tempo' ? 'https://api.tempo.io/4' : '',
      jiraAccountId: kind === 'tempo' ? free!.id : kind === 'confluence' ? (s.defaultJiraAccountId ?? '') : '',
    });
    return { settings: structuredClone(s), id, needsJira: false };
  }

  async removeProfile(id: string) {
    return this.change((s) => {
      s.jiraAccounts = s.jiraAccounts.filter((a) => a.id !== id);
      s.profiles = s.profiles.filter((p) => p.id !== id && !(p.kind === 'tempo' && p.jiraAccountId === id));
      if (!s.jiraAccounts.some((a) => a.id === s.defaultJiraAccountId))
        s.defaultJiraAccountId = s.jiraAccounts[0]?.id ?? null;
    });
  }

  async updateProfile(p: Profile) {
    return this.change((s) => (s.profiles = s.profiles.map((x) => (x.id === p.id ? { ...p, kind: x.kind } : x))));
  }

  async updateJiraAccount(a: JiraAccount) {
    return this.change((s) => (s.jiraAccounts = s.jiraAccounts.map((x) => (x.id === a.id ? a : x))));
  }

  async setDefaultJiraAccount(id: string) {
    return this.change((s) => (s.defaultJiraAccountId = id));
  }

  async checkJira(accountId: string): Promise<CheckLine[]> {
    const a = this.settingsValue.jiraAccounts.find((x) => x.id === accountId);
    if (this.opts.jiraCheckFails)
      return [
        { title: 'Signed in as me@example.com', detail: '', ok: true },
        { title: 'Reading issues failed (403)', detail: 'The token needs the read:jira-work scope.', ok: false },
      ];
    return [
      { title: `Signed in as ${a?.email || 'you'}`, detail: `Account id ${a?.accountId || '–'}`, ok: true },
      { title: 'Issues can be read', detail: 'ORB-214 found.', ok: true },
    ];
  }

  async checkTempo(): Promise<CheckLine[]> {
    return [
      { title: 'Tempo worklogs can be read', detail: '38 worklogs this month.', ok: true },
      { title: 'Sending worklogs', detail: 'Not checked: it would create a worklog.', ok: null },
    ];
  }

  async checkEmail(): Promise<CheckLine[]> {
    return [
      { title: 'Server, user name and password', detail: 'Signed in to imap.example.com:993.', ok: true },
      { title: 'Sent folder', detail: 'Reading "Sent" (read-only).', ok: true },
    ];
  }

  /** Two threads a day from the sample: one on the day's ticket (with its draft), a reply thread without one. */
  async sentEmails(from: string, to: string): Promise<EmailsDto> {
    if (this.opts.empty) return { days: [], mails: 0, threads: 0, profiles: 0 };
    const m = await this.month(from.slice(0, 7));
    const days: EmailsDto['days'] = [];
    const dayKeys = [...new Set(m.worklogs.map((w) => w.day))].filter((d) => d >= from && d <= to);
    for (const day of dayKeys.sort().reverse()) {
      const w = m.worklogs.find((x) => x.day === day && x.issueKey);
      if (!w?.issueKey) continue;
      days.push({
        day,
        threads: [
          {
            subject: 'Offsite agenda',
            times: ['13:05', '16:40'],
            subjects: ['Offsite agenda', 'RE: Offsite agenda'],
            issueKey: '',
            entry: null,
          },
          {
            subject: `${w.issueKey} pricing question`,
            times: ['09:20'],
            subjects: [`${w.issueKey} pricing question`],
            issueKey: w.issueKey,
            entry: {
              id: w.id,
              source: w.source,
              description: w.tempoText,
              seconds: 600,
              status: w.status,
              focus: w.focus,
            },
          },
        ],
      });
    }
    return { days, mails: days.length * 3, threads: days.length * 2, profiles: 1 };
  }

  async checkConfluence(): Promise<CheckLine[]> {
    return [
      { title: 'Site, email and API token', detail: 'Signed in.', ok: true },
      { title: 'Search pages (search:confluence)', detail: 'Pages can be searched.', ok: true },
    ];
  }

  async readProjects(accountId: string) {
    const keys = ['LEAVE', 'TEAM', 'ORB', 'NOVA'];
    this.settingsValue.jiraAccounts = this.settingsValue.jiraAccounts.map((a) =>
      a.id === accountId ? { ...a, projectKeys: keys } : a,
    );
    return keys;
  }

  // ── Export ─────────────────────────────────────────────────────────────────

  async exportText(ids: number[]) {
    const rows = this.each(ids).map(
      (w) => `- [ ] ${w.day} ${w.issueKey} ${Math.round(w.duration / 60)}m ${w.comment || w.description}`,
    );
    return `# Log these worklogs in Tempo\n\n${rows.join('\n')}\n`;
  }

  async exportFile(_ids: number[], format: ExportFormat) {
    return `~/Downloads/worklogs.${format === 'markdown' ? 'md' : format}`;
  }

  async pushDayToTempo(day: string) {
    const logs = this.get(day.slice(0, 7)).worklogs.filter((w) => w.day === day);
    const drafts = logs.filter((w) => w.status === 'draft' && w.duration > 0).length;
    if (drafts)
      throw new Error(`${day} still has ${drafts} draft entries. Review them before sending the day to Tempo.`);
    const ready = logs.filter((w) => w.status === 'reviewed' && w.tempoId == null && w.issueKey && w.duration > 0);
    let tempoId = 9000;
    for (const w of ready) Object.assign(w, { status: 'exported', locked: true, tempoId: tempoId++ });
    const n = ready.length;
    this.emit<CollectDone>(EVENTS.collectDone, {
      cancelled: false,
      failed: false,
      message: {
        code: 'push.dayDone',
        params: { n, day, already: 0 },
        text: `Sent ${n} worklogs for ${day} to Tempo (0 already there, left out).`,
      },
    });
  }

  /** The sample's reviewed entries in the range, by the backend's rule (pushPlan). */
  private pushPlan(from: string, to: string) {
    const byDay = new Map<string, WorklogDto[]>();
    for (const month of SAMPLE_MONTHS)
      for (const w of this.get(month).worklogs)
        if (w.day >= from && w.day <= to) byDay.set(w.day, [...(byDay.get(w.day) ?? []), w]);
    const ready: WorklogDto[] = [];
    const preview: PushPreviewDto = { entries: 0, seconds: 0, days: 0, draftDays: [], later: 0 };
    for (const [day, logs] of [...byDay].sort(([a], [b]) => a.localeCompare(b))) {
      const dayReady = logs.filter((w) => w.status === 'reviewed' && w.tempoId == null && w.issueKey && w.duration > 0);
      if (day > SAMPLE_TODAY) preview.later += dayReady.length;
      else if (logs.some((w) => w.status === 'draft' && w.duration > 0)) preview.draftDays.push(day);
      else if (dayReady.length) {
        ready.push(...dayReady);
        preview.days++;
        preview.entries += dayReady.length;
        preview.seconds += dayReady.reduce((n, w) => n + w.duration, 0);
      }
    }
    return { ready, preview };
  }

  async pushPreview(from: string, to: string) {
    return this.pushPlan(from, to).preview;
  }

  async pushRangeToTempo(from: string, to: string) {
    const { ready, preview } = this.pushPlan(from, to);
    let tempoId = 9500;
    for (const w of ready) Object.assign(w, { status: 'exported', locked: true, tempoId: tempoId++ });
    const n = ready.length;
    const drafts = preview.draftDays.length;
    this.emit<CollectDone>(EVENTS.collectDone, {
      cancelled: false,
      failed: false,
      message: {
        code: 'push.rangeDone',
        params: { n, from, to, already: 0, drafts, later: preview.later },
        text: `Sent ${n} worklogs for ${from} – ${to} to Tempo (0 already there, left out; ${drafts} days with drafts and ${preview.later} entries on later days not sent).`,
      },
    });
  }

  // ── Jobs and events ────────────────────────────────────────────────────────

  async cleanUpSettled(): Promise<CleanupDto> {
    return { days: 0, rows: 0 };
  }

  async collect(from: string, to: string, _force = false) {
    this.cancelled = false;
    for (const step of fixtures.collectSteps) {
      if (this.cancelled) break;
      this.emit<CollectProgress>(EVENTS.collectProgress, step);
      await new Promise((r) => setTimeout(r, this.opts.stepMs ?? 600));
    }
    this.emit<CollectDone>(EVENTS.collectDone, {
      cancelled: this.cancelled,
      failed: false,
      message: this.cancelled
        ? undefined
        : {
            code: 'collect.done',
            params: { commits: 7, meetings: 3, from, to },
            text: `Collected 7 commits and 3 meetings for ${from} – ${to}.`,
          },
    });
  }

  // Like Go's jobs, each one ends with collect:done (the store re-reads the month on it).
  async rebuild() {
    // The fake has nothing to rebuild from.
    this.emit<CollectDone>(EVENTS.collectDone, { cancelled: false, failed: false });
  }
  async fetchHrmaster() {
    this.cleared.sessions.clear();
    this.emit<CollectDone>(EVENTS.collectDone, { cancelled: false, failed: false });
  }
  async fetchGoogleCalendar() {
    // The fixtures already hold the meetings.
    this.emit<CollectDone>(EVENTS.collectDone, { cancelled: false, failed: false });
  }

  async fetchGoogleChat() {
    this.cleared.chats.clear();
    this.emit<CollectDone>(EVENTS.collectDone, { cancelled: false, failed: false });
  }

  async cancel() {
    this.cancelled = true;
  }

  async activity(): Promise<ActivityLine[]> {
    const t = '2026-11-28T09:00:00+01:00';
    return [
      {
        time: t,
        source: 'git',
        level: 'info',
        message: 'git -C ~/Workspaces/trip-planner log --all --author=m•••@example.com → 2048 bytes (84ms)',
        replace: false,
      },
      {
        time: t,
        source: 'http',
        level: 'error',
        message: 'GET https://api.tempo.io/4/worklogs?from=2026-11-01 → 401 (210ms)',
        replace: false,
      },
      { time: t, source: 'job', level: 'info', message: 'Collected 7 commits and 3 meetings.', replace: false },
    ];
  }

  onEvent<T>(name: string, cb: (data: T) => void) {
    const set = this.listeners.get(name) ?? new Set();
    set.add(cb as (data: unknown) => void);
    this.listeners.set(name, set);
    return () => set.delete(cb as (data: unknown) => void);
  }

  private emit<T>(name: string, data: T) {
    this.listeners.get(name)?.forEach((cb) => cb(data));
  }

  private get(month: string): MonthDto {
    let m = this.months.get(month);
    if (!m) this.months.set(month, (m = sampleMonth(month, this.opts.empty)));
    return m;
  }

  private each(ids: number[]): WorklogDto[] {
    return ids.map((id) => this.find(id)).filter((w): w is WorklogDto => !!w);
  }

  private find(id: number): WorklogDto | undefined {
    for (const m of this.months.values()) {
      const w = m.worklogs.find((l) => l.id === id);
      if (w) return w;
    }
    return undefined;
  }

  private remove(id: number) {
    for (const m of this.months.values()) m.worklogs = m.worklogs.filter((l) => l.id !== id);
  }
}

/** An entry's own flags from its fields, as WorklogDto describes them (no Tempo read-back in the fake). */
/** The sample's plans: each day's ticket with the most time, by day. */
function samplePlanned(logs: WorklogDto[]) {
  const time = new Map<string, Map<string, number>>();
  for (const w of logs) {
    if (!w.issueKey || w.duration <= 0) continue;
    const day = time.get(w.day) ?? new Map<string, number>();
    day.set(w.issueKey, (day.get(w.issueKey) ?? 0) + w.duration);
    time.set(w.day, day);
  }
  const out = new Map<string, string>();
  for (const day of [...time.keys()].sort()) {
    const [key] = [...time.get(day)!].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))[0];
    out.set(day, key);
  }
  return out;
}

function entryFlags(w: WorklogDto) {
  w.needsIssue = w.issueKey === null && w.duration > 0;
  w.needsComment = w.status === 'draft' && !w.comment.trim();
  w.tempoText = w.comment.trim() ? w.comment : w.description;
  w.inTempo = w.status === 'exported';
  w.effectiveStatus = w.status;
  w.canDelete = w.tempoId === null;
  w.canMove = w.status !== 'exported';
}

function issueOption(key: string, summary: string): IssueOptionDto {
  return {
    key,
    summary,
    origin: 'recent',
    day: '',
    uses: 0,
    status: '',
    description: '',
    account: '',
    accountName: '',
  };
}
