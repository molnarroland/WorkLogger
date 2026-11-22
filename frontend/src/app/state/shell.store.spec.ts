import { TestBed } from '@angular/core/testing';
import { SnAlert } from '@spartan-native/desktop-ui';
import { API } from '../api/api';
import { FakeApi, type FakeApiOptions } from '../api/fake-api';
import { fixtures } from '../api/testing/fixtures';
import { I18N_LANG } from '../i18n/t.pipe';
import { ShellStore } from './shell.store';

// The store holds what the backend computed; the FakeApi's mutations are naive (they change an entry's own
// fields, not the derived totals), so these specs check the wiring: the call is made and the month re-read.
// The derived values themselves are tested in Go (internal/view).
async function store(
  opts: FakeApiOptions = {},
  api = new FakeApi({ stepMs: 0, ...opts }),
  confirm = vi.fn(async () => true),
) {
  TestBed.configureTestingModule({
    providers: [
      { provide: API, useValue: api },
      { provide: SnAlert, useValue: { confirm } },
    ],
  });
  const s = TestBed.inject(ShellStore);
  await s.ready;
  return s;
}

describe('ShellStore', () => {
  it('asks before sending a range to Tempo, with the days left out, then sends it as a job', async () => {
    const api = new FakeApi({ stepMs: 0 });
    const confirm = vi.fn(async () => true);
    const s = await store({}, api, confirm);
    const send = vi.spyOn(api, 'pushRangeToTempo');
    vi.spyOn(api, 'pushPreview').mockResolvedValue({
      entries: 3,
      seconds: 3 * 3600,
      days: 2,
      draftDays: ['2026-10-29'],
      later: 1,
    });
    await s.sendRange('2026-10-26', '2026-11-01');
    expect(confirm).toHaveBeenCalledTimes(1);
    const [detail, o] = confirm.mock.calls[0] as unknown as [string, { title: string }];
    expect(o.title).toBe('Send 3 reviewed worklogs (3h) on 2 days for 2026-10-26 – 2026-11-01 to Tempo?');
    expect(detail).toMatch(/1 day still has drafts and is left out: .*29/);
    expect(detail).toMatch(/1 entry on a day that hasn't come yet is left out/);
    expect(send).toHaveBeenCalledWith('2026-10-26', '2026-11-01');
    expect(s.busy()).toBe(false);
  });

  it("doesn't send when the question is cancelled, nor ask when nothing is reviewed", async () => {
    const api = new FakeApi({ stepMs: 0 });
    const confirm = vi.fn(async () => false);
    const s = await store({}, api, confirm);
    const send = vi.spyOn(api, 'pushRangeToTempo');
    const preview = vi.spyOn(api, 'pushPreview');
    preview.mockResolvedValue({ entries: 1, seconds: 3600, days: 1, draftDays: [], later: 0 });
    await s.sendRange('2026-10-01', '2026-10-31');
    expect(send).not.toHaveBeenCalled();
    preview.mockResolvedValue({ entries: 0, seconds: 0, days: 0, draftDays: [], later: 0 });
    await s.sendRange('2026-10-01', '2026-10-31');
    expect(confirm).toHaveBeenCalledTimes(1);
    expect(s.status()).toBe('Nothing reviewed to send for 2026-10-01 – 2026-10-31.');
  });

  it("loads modules and the backend's current month when unlocked", async () => {
    const s = await store();
    expect(s.locked()).toBe(false);
    expect(s.month()).toBe('2026-10');
    expect(s.today()).toBe('2026-10-30');
    expect(s.range()).toEqual({ from: '2026-10-01', to: '2026-10-31' });
    expect(s.attention().map((d) => d.day)).toEqual(['2026-10-27', '2026-10-29', '2026-10-30']);
    expect(s.panes().map((p) => p.id)).toContain('chats');
    expect(s.days().size).toBe(31);
    expect(s.needsTicket()).toBe(1); // the Code review on the 27th
    expect(s.progress().total).toBeGreaterThan(s.progress().entered);
  });

  it('hides panes of switched-off modules', async () => {
    const ids = (await store({ settings: fixtures.gitOnly })).panes().map((p) => p.id);
    expect(ids).toContain('repositories');
    expect(ids).not.toContain('meetings');
    expect(ids).not.toContain('sessions');
    expect(ids).not.toContain('chats');
  });

  it('stays locked and says so when unlock fails', async () => {
    const s = await store({ locked: true, unlockSucceeds: false });
    await s.unlock();
    expect(s.locked()).toBe(true);
    expect(s.stillLocked()).toBe(true);
    expect(s.data()).toBeNull();
  });

  it('unlocks and loads', async () => {
    const s = await store({ locked: true });
    await s.unlock();
    expect(s.locked()).toBe(false);
    expect(s.worklogs().length).toBeGreaterThan(0);
  });

  it('shows collect progress, then the summary, and clears busy when done', async () => {
    const s = await store();
    const seen: string[] = [];
    const run = s.run('collect');
    expect(s.busy()).toBe(true);
    const poll = setInterval(() => seen.push(s.status()), 0);
    await run;
    clearInterval(poll);
    expect(s.busy()).toBe(false);
    expect(seen).toContain('Building drafts…');
    // The last progress step ("Building drafts…") must not stay up as if the job hung.
    expect(s.status()).toBe('Collected 7 commits and 3 meetings for 2026-10-01 – 2026-10-31.');
  });

  it('locks an edited entry', async () => {
    const s = await store();
    const w = s.worklogs().find((l) => l.status === 'draft' && !l.locked)!;
    await s.updateWorklog({ ...w, duration: w.duration + 900 });
    const saved = s.worklogs().find((l) => l.id === w.id)!;
    expect(saved.duration).toBe(w.duration + 900);
    expect(saved.locked).toBe(true);
  });

  it('moves entries between days but leaves exported ones', async () => {
    const s = await store();
    const draft = s.worklogs().find((l) => l.status === 'draft')!;
    const exported = s.worklogs().find((l) => l.status === 'exported' && l.source === 'git')!;
    await s.moveWorklogs([draft.id, exported.id], '2026-10-29');
    expect(s.worklogs().find((l) => l.id === draft.id)!.day).toBe('2026-10-29');
    expect(s.worklogs().find((l) => l.id === exported.id)!.day).toBe(exported.day);
  });

  it('gives the no-ticket entry a ticket', async () => {
    const s = await store();
    const missing = s.worklogs().find((l) => l.needsIssue)!;
    await s.setIssue([missing.id], 'ORB-9');
    expect(s.worklogs().find((l) => l.id === missing.id)!.issueKey).toBe('ORB-9');
  });

  it('ticks a ticket-day off and back', async () => {
    const s = await store();
    const day = s
      .tickets()
      .find((g) => g.key)!
      .days.find((d) => !d.entered)!;
    const status = () =>
      s
        .worklogs()
        .filter((w) => day.ids.includes(w.id))
        .map((w) => w.status);
    await s.setEntered(day.ids, true);
    expect(new Set(status())).toEqual(new Set(['exported']));
    await s.setEntered(day.ids, false);
    expect(new Set(status())).toEqual(new Set(['reviewed']));
  });

  it('deletes entries in one call', async () => {
    const s = await store();
    const drafts = s
      .worklogs()
      .filter((w) => w.canDelete && w.status === 'draft')
      .slice(0, 2);
    await s.deleteWorklogs(drafts.map((w) => w.id));
    expect(s.worklogs().some((w) => drafts.some((d) => d.id === w.id))).toBe(false);
  });

  it('switches months to what the backend says comes before and after', async () => {
    const s = await store();
    await s.setMonth(s.data()!.next);
    expect(s.month()).toBe('2026-11');
    expect(s.range().to).toBe('2026-11-30');
  });

  it('saves a settings change and keeps the rest', async () => {
    const s = await store();
    const before = s.settings()!;
    await s.saveSettings({ chatGapMinutes: 20 });
    expect(s.settings()!.chatGapMinutes).toBe(20);
    expect(s.settings()!.meetingRules).toEqual(before.meetingRules);
  });

  it('drops panes when a module is switched off in settings', async () => {
    const s = await store();
    await s.saveSettings({ modules: ['git'] });
    expect(s.panes().map((p) => p.id)).not.toContain('chats');
  });

  it('reads Tempo back after loading and reloads the month only when that changed it', async () => {
    const api = new FakeApi({ stepMs: 0 });
    const refresh = vi.spyOn(api, 'refreshTempo').mockResolvedValue(true);
    const month = vi.spyOn(api, 'month');
    const s = await store({}, api);
    await vi.waitFor(() => expect(month).toHaveBeenCalledTimes(2));
    expect(refresh).toHaveBeenCalledWith('2026-10');

    refresh.mockResolvedValue(false);
    await s.setMonth('2026-11');
    await vi.waitFor(() => expect(refresh).toHaveBeenLastCalledWith('2026-11'));
    expect(month).toHaveBeenCalledTimes(3);
  });

  it('puts a failed read-back in the status line', async () => {
    const api = new FakeApi({ stepMs: 0 });
    vi.spyOn(api, 'refreshTempo').mockRejectedValue(new Error('Reading back from Tempo failed'));
    const s = await store({}, api);
    await vi.waitFor(() => expect(s.status()).toBe('Reading back from Tempo failed'));
  });

  it('lists each job as a task: running, then how collect:done says it ended', async () => {
    const api = new FakeApi({ stepMs: 0 });
    const s = await store({}, api);
    const run = s.run('collect');
    expect(s.tasks().at(-1)).toMatchObject({ label: 'Collect range', state: 'running' });
    await run;
    expect(s.tasks().find((t) => t.label === 'Collect range')).toMatchObject({ state: 'done' });

    vi.spyOn(api, 'fetchHrmaster').mockRejectedValue(new Error('HRmaster: could not reach 2026-10.'));
    await s.run('fetchHrmaster');
    expect(s.tasks().find((t) => t.label === 'Fetch work times from HRmaster')).toMatchObject({
      state: 'failed',
      message: 'HRmaster: could not reach 2026-10.',
    });
    s.clearTasks();
    expect(s.tasks().filter((t) => t.state !== 'running')).toEqual([]);
  });

  it('runs a job over a period instead of the month', async () => {
    const api = new FakeApi({ stepMs: 0 });
    const s = await store({}, api);
    const spy = vi.spyOn(api, 'fetchGoogleChat');
    await s.run('fetchGoogleChat', { from: '2026-10-19', to: '2026-10-25' });
    expect(spy).toHaveBeenCalledWith('2026-10-19', '2026-10-25');
    expect(s.tasks().some((t) => t.label.endsWith('(2026-10-19 – 2026-10-25)'))).toBe(true);
  });

  it('shows backend messages and user errors in the UI language', async () => {
    const api = new FakeApi({ stepMs: 0 });
    TestBed.configureTestingModule({ providers: [{ provide: I18N_LANG, useValue: 'hu' }] });
    vi.spyOn(api, 'setIssue').mockRejectedValue(
      Object.assign(new Error('Enter an issue key.'), {
        cause: { code: 'issue.missing', text: 'Enter an issue key.' },
      }),
    );
    const s = await store({}, api);
    await s.setIssue([1], '');
    expect(s.status()).toBe('Adj meg egy jegykulcsot.');
    await s.run('collect');
    expect(s.status()).toBe('7 commit és 3 megbeszélés összegyűjtve: 2026-10-01 – 2026-10-31.');
  });

  it('shows an error without a code as it is', async () => {
    const api = new FakeApi({ stepMs: 0 });
    vi.spyOn(api, 'setIssue').mockRejectedValue(new Error('database is locked'));
    const s = await store({}, api);
    await s.setIssue([1], 'ORB-1');
    expect(s.status()).toBe('database is locked');
  });

  it("runs a page's call and puts its failure in the status line", async () => {
    const s = await store();
    expect(await s.attempt(() => Promise.resolve(3))).toBe(3);
    expect(await s.attempt(() => Promise.reject(new Error('disk full')))).toBeUndefined();
    expect(s.status()).toBe('disk full');
  });
});
