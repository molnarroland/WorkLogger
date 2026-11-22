import { DestroyRef, Injectable, type Signal, computed, inject, signal } from '@angular/core';
import { SnAlert } from '@spartan-native/desktop-ui';
import {
  API,
  type Api,
  type ActivityLine,
  type CollectDone,
  type CollectProgress,
  type DayDto,
  EVENTS,
  type MonthDto,
  type SettingsDto,
  type SetupStepDto,
  type Status,
  type WorklogDto,
} from '../api/api';
import { I18n } from '../i18n/t.pipe';
import { visiblePanes } from '../shell/panes';
import type { Task, TaskState } from '../shared/task';
import { dayTitle, formatDuration } from '../shared/worklogs';

export type Job = 'collect' | 'rebuild' | 'fetchHrmaster' | 'fetchGoogleChat' | 'fetchGoogleCalendar';

const JOB_LABEL: Record<Job, string> = {
  collect: 'toolbar.collectRange',
  rebuild: 'toolbar.rebuild',
  fetchHrmaster: 'toolbar.fetchHrmaster',
  fetchGoogleChat: 'toolbar.fetchChat',
  fetchGoogleCalendar: 'tasks.fetchCalendar',
};

/** What the header line's centre shows: a period and its logged and expected time (seconds). */
export interface Heading {
  title: string;
  logged: number;
  expected: number;
}

/**
 * A page's part of the header line: its step through its days (drawn inside the month pager; the labels are i18n
 * keys), and its own heading instead of the month's.
 */
export interface PageHeader {
  previous: string;
  next: string;
  step: (by: -1 | 1) => unknown;
  /** How many days one step moves, shown on the step buttons (+1 a day, +7 a week). */
  stepDays: number;
  /** The calendar button: back to the page's own today (this day, this week) instead of this month. */
  today: { label: string; icon: string; go: () => unknown; current: Signal<boolean> };
  /** The calendar after the pager: picks a day, or a week by any of its days; label is an i18n key. */
  pick: { label: string; mode: 'day' | 'week'; value: Signal<string>; go: (day: string) => unknown };
  heading?: Signal<Heading | null>;
}

/** Puts a page's part in the header line while the page is open (call in its constructor). */
export function providePageHeader(store: ShellStore, destroy: DestroyRef, header: PageHeader) {
  store.pageHeader.set(header);
  destroy.onDestroy(() => {
    if (store.pageHeader() === header) store.pageHeader.set(null); // the next page may have set its own already
  });
}

/**
 * Window-wide state: lock, modules, the selected month as the backend computed it (days with their states,
 * worklogs, to-dos, tickets), and the one long job that may be running. Timesheet, Review, By ticket and Totals
 * all read the same month, so it lives here, once. Nothing here computes domain values: it holds what Go sent.
 */
@Injectable({ providedIn: 'root' })
export class ShellStore {
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);
  private readonly alert = inject(SnAlert);

  readonly locked = signal(true);
  readonly unlocking = signal(false);
  readonly stillLocked = signal(false);
  readonly settings = signal<SettingsDto | null>(null);
  readonly modules = computed(() => this.settings()?.modules ?? []);
  /** What the chosen modules still need (the backend's setup steps); re-read with every settings save. */
  readonly setup = signal<SetupStepDto[]>([]);
  /** The selected "YYYY-MM"; empty until the first load, which asks the backend for the current month. */
  readonly month = signal('');
  readonly data = signal<MonthDto | null>(null);
  /** Counts the month's finished Tempo read-backs, so pages built from them (Plans) read again. */
  readonly tempoReads = signal(0);
  /** The open page's part of the header line (its step, its own title); null on pages without one. */
  readonly pageHeader = signal<PageHeader | null>(null);
  readonly busy = signal(false);
  readonly status = signal('');
  /** The activity console: this session's background work, as the backend logged (and masked) it. */
  readonly activity = signal<ActivityLine[]>([]);
  /** This session's tasks, newest last (the status bar's Tasks popup); the last 50 are kept. */
  readonly tasks = signal<Task[]>([]);
  private nextTask = 1;
  /** The job task collect:done ends (0: none). */
  private jobTask = 0;
  /** Until when (ms) Collect forces the weeks the last one left out as settled: pressing it again fetches them. */
  private forceUntil = 0;

  readonly panes = computed(() => visiblePanes(this.modules()));
  /** The ids of the switched-on pages, for links that open one (shared/focus.ts). */
  readonly paneIds = computed(() => this.panes().map((p) => p.id));
  /** The backend's today and its month. */
  readonly today = computed(() => this.data()?.today ?? '');
  readonly currentMonth = computed(() => this.data()?.currentMonth ?? '');
  /** The month's first and last day; empty until loaded. Changes only with the month, not with every edit. */
  readonly range = computed(() => ({ from: this.data()?.from ?? '', to: this.data()?.to ?? '' }), {
    equal: (a, b) => a.from === b.from && a.to === b.to,
  });
  readonly worklogs = computed(() => this.data()?.worklogs ?? []);
  readonly days = computed(() => new Map((this.data()?.days ?? []).map((d) => [d.day, d])));
  readonly tickets = computed(() => this.data()?.tickets ?? []);
  readonly needsTicket = computed(() => this.data()?.needsTicket ?? 0);
  readonly progress = computed(() => this.data()?.tempoProgress ?? { entered: 0, total: 0 });
  readonly summary = computed(() => this.data()?.summary ?? null);
  readonly todos = computed(() => this.data()?.todos ?? []);
  /** The side panel's lists, and the Days view's order. */
  readonly attention = computed(() => this.dayList(this.data()?.attention));
  readonly drafts = computed(() => this.dayList(this.data()?.drafts));
  readonly queue = computed(() => this.data()?.queue ?? []);

  /** Resolves once the lock state (and, when unlocked, the month) is known. */
  readonly ready: Promise<void>;

  constructor() {
    const off = this.api.onEvent<CollectProgress>(EVENTS.collectProgress, (p) =>
      this.status.set(this.i18n.say(p.message)),
    );
    // A job's last word replaces its progress ("Building drafts…" would otherwise stay up as if it hung).
    // The month is re-read here, not when the job's call returns: a long call (an HRmaster fetch) can settle
    // early in the webview, and the open page would keep showing the month from before the job.
    const offDone = this.api.onEvent<CollectDone>(EVENTS.collectDone, (d) => {
      this.status.set(d.message?.text ? this.i18n.say(d.message) : d.cancelled ? this.i18n.t('msg.job.cancelled') : '');
      const n = d.settled ?? 0;
      if (n) {
        this.status.update((s) => `${s} ${this.i18n.t('collect.settled', { n })}`.trim());
        this.forceUntil = Date.now() + 60_000;
      }
      if (this.jobTask)
        this.endTask(this.jobTask, d.cancelled ? 'cancelled' : d.failed ? 'failed' : 'done', this.status());
      this.jobTask = 0;
      void this.reload().then(() => this.refreshTempo());
    });
    const offActivity = this.api.onEvent<ActivityLine>(EVENTS.activity, (l) =>
      this.activity.update((list) =>
        (l.replace && list.length ? [...list.slice(0, -1), l] : [...list, l]).slice(-2000),
      ),
    );
    inject(DestroyRef).onDestroy(() => (off(), offDone(), offActivity()));
    this.ready = this.init();
  }

  private async init() {
    this.activity.set(await this.api.activity());
    this.locked.set(await this.api.isLocked());
    if (!this.locked()) await this.load();
  }

  async unlock() {
    this.unlocking.set(true);
    try {
      const ok = await this.api.unlock();
      this.locked.set(!ok);
      this.stillLocked.set(!ok);
      if (ok) await this.load();
    } finally {
      this.unlocking.set(false);
    }
  }

  async setMonth(month: string) {
    this.month.set(month);
    await this.reload();
    void this.refreshTempo();
  }

  /**
   * One job at a time; a second request while busy is ignored (Collect is disabled meanwhile). It covers the
   * month, or `period` (a day or a week, e.g. Google Chat for the page's own days).
   */
  async run(job: Job, period?: { from: string; to: string }) {
    if (this.busy() || !this.data()) return;
    const { from, to } = period ?? this.range();
    this.busy.set(true);
    const force = job === 'collect' && Date.now() < this.forceUntil;
    this.forceUntil = 0;
    const label =
      this.i18n.t(JOB_LABEL[job]) +
      (period ? ` (${from === to ? from : `${from} – ${to}`})` : '') +
      (force ? ` · ${this.i18n.t('collect.forced')}` : '');
    const task = (this.jobTask = this.startTask(label));
    try {
      // The month is re-read on the job's collect:done.
      await (job === 'collect' ? this.api.collect(from, to, force) : this.api[job](from, to));
    } catch (e) {
      this.fail(e);
      this.endTask(task, 'failed', this.status()); // collect:done, if it still comes, has the last word
    } finally {
      this.busy.set(false);
    }
  }

  /** Deletes the local source data of the month's settled weeks (worklogs stay), then re-reads the month. */
  async cleanUpSettled() {
    const { from, to } = this.range();
    if (this.busy() || !from) return;
    const r = await this.attempt(() => this.api.cleanUpSettled(from, to));
    if (!r) return;
    this.status.set(this.i18n.t('cleanup.done', { days: r.days, rows: r.rows }));
    await this.reload();
  }

  /** Sends one day to Tempo, by hand (the job's last word goes to the status bar); a job slot like run(). */
  pushDay(day: string) {
    return this.push(this.i18n.t('tasks.pushDay', { day }), () => this.api.pushDayToTempo(day));
  }

  /**
   * Sends the reviewed entries of a week or a month (inclusive) to Tempo, after asking: the question says how
   * many go and which days are left out (drafts, days to come). With nothing to send it only says so.
   */
  async sendRange(from: string, to: string) {
    if (this.busy() || !this.data()) return;
    const p = await this.attempt(() => this.api.pushPreview(from, to));
    if (!p) return;
    const lang = this.i18n.lang();
    const left = [
      p.draftDays.length
        ? this.i18n.t('push.rangeDrafts', {
            n: p.draftDays.length,
            days: p.draftDays.map((d) => dayTitle(d, lang)).join(', '),
          })
        : '',
      p.later ? this.i18n.t('push.rangeLater', { n: p.later }) : '',
    ].filter(Boolean);
    if (!p.entries) {
      this.status.set([this.i18n.t('push.rangeNothing', { from, to }), ...left].join(' '));
      return;
    }
    const yes = await this.alert.confirm([this.i18n.t('push.rangeDetail'), ...left].join(' '), {
      title: this.i18n.t('push.rangeQuestion', { n: p.entries, d: formatDuration(p.seconds), days: p.days, from, to }),
      confirmLabel: this.i18n.t('export.send'),
      cancelLabel: this.i18n.t('status.cancel'),
    });
    if (yes) await this.push(this.i18n.t('tasks.pushRange', { from, to }), () => this.api.pushRangeToTempo(from, to));
  }

  /** A send to Tempo as a job: the Tasks entry, the busy slot, then the month and Tempo's read-back again. */
  private async push(label: string, op: () => Promise<void>) {
    if (this.busy() || !this.data()) return;
    this.busy.set(true);
    const task = (this.jobTask = this.startTask(label));
    try {
      await op();
      await this.reload();
      void this.refreshTempo();
    } catch (e) {
      this.fail(e);
      this.endTask(task, 'failed', this.status());
    } finally {
      this.busy.set(false);
    }
  }

  cancel() {
    return this.api.cancel();
  }

  /** Empties the Tasks popup, keeping what still runs. */
  clearTasks() {
    this.tasks.update((list) => list.filter((t) => t.state === 'running'));
  }

  private startTask(label: string) {
    const id = this.nextTask++;
    const t: Task = { id, label, state: 'running', message: '', started: new Date().toISOString() };
    this.tasks.update((list) => [...list, t].slice(-50));
    return id;
  }

  private endTask(id: number, state: TaskState, message = '') {
    const ended = new Date().toISOString();
    this.tasks.update((list) => list.map((t) => (t.id === id ? { ...t, state, message, ended } : t)));
  }

  // Edits. Each one saves straight away, then re-reads the month so every page sees the same truth.
  updateWorklog = (w: WorklogDto) => this.edit(() => this.api.updateWorklog(w));
  setDuration = (id: number, text: string) => this.edit(() => this.api.setDuration(id, text));
  moveWorklogs = (ids: number[], toDay: string) => this.edit(() => this.api.moveWorklogs(ids, toDay));
  addWorklog = (day: string) => this.edit(() => this.api.addWorklog(day));
  deleteWorklogs = (ids: number[]) => this.edit(() => this.api.deleteWorklogs(ids));
  setStatus = (ids: number[], status: Status) => this.edit(() => this.api.setStatus(ids, status));
  setIssue = (ids: number[], key: string) => this.edit(() => this.api.setIssue(ids, key));
  setComment = (ids: number[], comment: string) => this.edit(() => this.api.setComment(ids, comment));
  useDescriptionsAsComments = (day: string) => this.edit(() => this.api.useDescriptionsAsComments(day));
  setEntered = (ids: number[], entered: boolean) => this.edit(() => this.api.setEntered(ids, entered));

  private async edit(op: () => Promise<unknown>) {
    try {
      await op();
    } catch (e) {
      this.fail(e);
    }
    await this.reload();
  }

  /** Re-reads the open month. */
  async reload() {
    const month = this.month();
    const data = await this.api.month(month);
    if (this.month() !== month) return; // a quick month switch may have overtaken this read
    this.data.set(data);
    if (!month) this.month.set(data.month);
  }

  private dayList(keys: readonly string[] = []): DayDto[] {
    const days = this.days();
    return keys.map((k) => days.get(k)).filter((d): d is DayDto => !!d);
  }

  /** Puts what went wrong in the status bar, in the UI language (backend user errors carry a code). */
  report(e: unknown) {
    const message = this.i18n.error(e);
    this.status.set(message);
    // Also in the activity console, where the whole message can be read (the status bar truncates it).
    const line: ActivityLine = {
      time: new Date().toISOString(),
      source: 'app',
      level: 'error',
      message,
      replace: false,
    };
    this.activity.update((list) => [...list, line].slice(-2000));
  }

  private fail(e: unknown) {
    this.report(e);
  }

  /**
   * Runs a page's API call and puts a failure in the status bar (RELEASE_REVIEW D6): its result, or undefined
   * when it failed. Every call a page makes outside the store's own edits goes through here, so none is an
   * unhandled rejection.
   */
  async attempt<T>(op: () => Promise<T>): Promise<T | undefined> {
    try {
      return await op();
    } catch (e) {
      this.report(e);
      return undefined;
    }
  }

  /** A settings edit by intent (Api.addPeriod, …): the backend saves it and answers with the settings. */
  async editSettings(op: (api: Api) => Promise<SettingsDto>) {
    try {
      this.settings.set(await op(this.api));
      // The month shows what settings decide (which period books a day), so it is re-read too.
      const [setup] = await Promise.all([this.api.setupSteps(), this.reload()]);
      this.setup.set(setup);
    } catch (e) {
      this.fail(e);
    }
  }

  /** Books every meeting whose title contains `title` to the ticket from now on, and rebuilds the month's drafts. */
  async alwaysForMeeting(title: string, issueKey: string) {
    await this.editSettings((api) => api.addMeetingRule(title, issueKey));
    await this.run('rebuild');
  }

  /** Merges a change into the flat preference fields and saves it straight away. */
  async saveSettings(patch: Partial<SettingsDto>) {
    const current = this.settings();
    if (!current) return;
    this.settings.set({ ...current, ...patch }); // optimistic: inputs don't jump back while saving
    try {
      this.settings.set(await this.api.saveSettings({ ...current, ...patch }));
      this.setup.set(await this.api.setupSteps());
    } catch (e) {
      this.settings.set(current);
      this.fail(e);
    }
  }

  /**
   * What Tempo already has, read back in the background: the month shows at once, and again when the read-back
   * changed it. A failure goes to the status line.
   */
  async refreshTempo() {
    const month = this.month();
    if (!month) return;
    const task = this.startTask(this.i18n.t('tasks.readTempo', { month }));
    try {
      if ((await this.api.refreshTempo(month)) && this.month() === month) await this.reload();
      this.tempoReads.update((n) => n + 1);
      this.endTask(task, 'done');
    } catch (e) {
      this.fail(e);
      this.endTask(task, 'failed', this.i18n.error(e));
    }
  }

  private async load() {
    const [settings, setup] = await Promise.all([this.api.settings(), this.api.setupSteps(), this.reload()]);
    this.settings.set(settings);
    this.setup.set(setup);
    void this.refreshTempo();
  }
}
