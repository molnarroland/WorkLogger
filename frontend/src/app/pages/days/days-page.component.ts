import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  linkedSignal,
  resource,
  untracked,
} from '@angular/core';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  lucideArrowRight,
  lucideCalendarDays,
  lucideCalendarX,
  lucideCheck,
  lucideMessagesSquare,
  lucidePlus,
  lucideSend,
} from '@ng-icons/lucide';
import {
  SnAlert,
  SnCommandBarImports,
  SnInspectorImports,
  SnPageCommandsTemplate,
  SnResizable,
  SnStatusPage,
} from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { API, EVENTS, type Focus } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { TRIAGE_COLOR, toBarParts } from '../../shared/colors';
import { formatDuration, parseDay } from '../../shared/worklogs';
import { ShellStore, providePageHeader } from '../../state/shell.store';
import { DayEntriesComponent } from '../../ui/day-entries/day-entries.component';
import { openFocus } from '../../shared/focus';
import { DayEvidenceComponent } from '../../ui/day-evidence/day-evidence.component';
import { SourceBarComponent } from '../../ui/source-bar/source-bar.component';
import { WorklogInspectorComponent } from '../../ui/worklog-inspector/worklog-inspector.component';

const DAY_KEY = /^\d{4}-\d{2}-\d{2}$/;

/**
 * One day at a time (layout C): its entries with what blocks Tempo fixed in place, and beside them what was
 * collected that day (meetings, chats), whose tickets can be used with one click. The side panel lists the days
 * that need it; "Mark reviewed and go to next" walks that list. `/days` alone opens the first day left to do.
 */
@Component({
  selector: 'wl-days-page',
  imports: [
    SnResizable,
    DayEntriesComponent,
    DayEvidenceComponent,
    DurationPipe,
    HlmButtonImports,
    NgIcon,
    SnPageCommandsTemplate,
    SnCommandBarImports,
    SnInspectorImports,
    SnStatusPage,
    SourceBarComponent,
    TPipe,
    WorklogInspectorComponent,
  ],
  providers: [
    provideIcons({
      lucideArrowRight,
      lucideCalendarDays,
      lucideCalendarX,
      lucideCheck,
      lucideMessagesSquare,
      lucidePlus,
      lucideSend,
    }),
  ],
  templateUrl: './days-page.component.html',
  styleUrl: './days-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DaysPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly router = inject(Router);
  /** Opens where an entry or a meeting / chat came from (the panel's "Show in …"). */
  protected readonly open = (f: Focus) => openFocus(this.router, f);
  private readonly i18n = inject(I18n);
  private readonly alert = inject(SnAlert);

  /** The route's day ("/days/2026-10-27"); none: the first day left to do. */
  readonly day = input<string>();
  /** `?focus=<id>`: the entry to select (the week view's blocks link here). */
  readonly focus = input<string>();

  protected readonly current = computed(() => {
    const day = this.day();
    if (day && DAY_KEY.test(day)) return day;
    return this.store.data()?.openDay ?? '';
  });

  protected readonly info = computed(() => this.store.days().get(this.current()));
  /** The day's entries (the backend sends them by start time). */
  protected readonly logs = computed(() => this.store.worklogs().filter((w) => w.day === this.current()));
  protected readonly logged = computed(() => this.info()?.logged ?? 0);
  protected readonly parts = computed(() => toBarParts(this.info()?.parts ?? []));
  protected readonly color = computed(() => TRIAGE_COLOR[this.info()?.state ?? 'free']);
  protected readonly title = computed(() =>
    this.current()
      ? new Intl.DateTimeFormat(this.i18n.lang(), { weekday: 'long', month: 'long', day: 'numeric' }).format(
          parseDay(this.current()),
        )
      : '',
  );

  /** The selected entry, for the inspector. Reset when the day changes: to `?focus=`, else none. */
  protected readonly selected = linkedSignal({
    source: () => ({ day: this.current(), focus: this.focus() }),
    computation: ({ focus }): number | null => (focus ? Number(focus) || null : null),
  });
  protected readonly inspected = computed(() => this.logs().find((w) => w.id === this.selected()) ?? null);
  /** The inspector column: closed by hand, it comes back with the next day or selection. */
  protected readonly panelOpen = linkedSignal(() => (this.current(), this.selected(), true));

  protected readonly drafts = computed(() => this.logs().filter((w) => w.status === 'draft'));
  protected readonly noComment = computed(() => this.logs().filter((w) => w.needsComment));
  protected readonly missing = computed(() => this.logs().filter((w) => w.needsIssue));
  /**
   * What "Send day to Tempo" sends: the reviewed entries with a ticket and time that aren't in Tempo yet. Sending
   * is by hand and one day at a time; the backend also leaves out what Tempo already has for the day.
   */
  protected readonly sendable = computed(() =>
    this.logs().filter((w) => w.status === 'reviewed' && w.tempoId == null && !!w.issueKey && w.duration > 0),
  );
  protected readonly sendableSeconds = computed(() => this.sendable().reduce((n, w) => n + w.duration, 0));
  /** Only past days and today, with every draft reviewed. */
  protected readonly canSend = computed(
    () =>
      this.sendable().length > 0 &&
      !this.drafts().length &&
      !!this.current() &&
      this.current() <= this.store.today() &&
      !this.store.busy(),
  );

  /** The next day in the review queue. */
  protected readonly next = computed(() => this.info()?.next ?? null);

  /** Where a ticket from "What happened" goes: the selected entry, else the day's entries without one. */
  protected readonly ticketTargets = computed(() => {
    const w = this.inspected();
    if (w) return w.canMove ? [w.id] : [];
    return this.missing().map((x) => x.id);
  });

  /** Meetings and chats of the day, for the modules that are on. */
  protected readonly evidence = resource({
    params: () => ({ day: this.current(), modules: this.store.modules(), locked: this.store.locked() }),
    loader: async ({ params: { day, modules, locked } }) => {
      if (locked || !day) return { meetings: [], chats: [] };
      // The backend returns what starts on the day.
      const [meetings, chats] = await Promise.all([
        modules.includes('calendar') ? this.api.meetings(day, day).then((m) => m.meetings) : Promise.resolve([]),
        modules.includes('chats')
          ? this.api.chats(day, day).then((c) => c.days.flatMap((d) => d.chats))
          : Promise.resolve([]),
      ]);
      return { meetings, chats };
    },
  });

  /** The day's chats (conversations), once collected. */
  protected readonly chatCount = computed(() => this.evidence.value()?.chats.length ?? 0);
  /** A past day whose chats are in: the button opens them instead of reading Google Chat again. Today still grows. */
  protected readonly chatsCollected = computed(() => this.chatCount() > 0 && this.current() < this.store.today());

  protected chatCommand() {
    const day = this.current();
    if (this.chatsCollected()) return openFocus(this.router, { pane: 'chats', day, id: '' });
    return this.store.run('fetchGoogleChat', { from: day, to: day });
  }

  constructor() {
    const destroy = inject(DestroyRef);
    // A fetch brings the day's chats in: count them again.
    destroy.onDestroy(this.api.onEvent<string>(EVENTS.chatDay, () => this.evidence.reload()));
    providePageHeader(this.store, destroy, {
      previous: 'days.previousDay',
      next: 'days.nextDay',
      step: (by) => this.step(by),
      stepDays: 1,
      today: {
        label: 'days.today',
        icon: 'lucideCalendar1',
        go: () => this.go(this.store.today()),
        current: computed(() => this.current() === this.store.today()),
      },
      pick: { label: 'days.pick', mode: 'day', value: this.current, go: (day) => this.go(day) },
    });
    // A day in another month (a link, the week view): bring that month in.
    effect(() => {
      const day = this.day();
      if (day && DAY_KEY.test(day) && day.slice(0, 7) !== untracked(this.store.month))
        this.store.setMonth(day.slice(0, 7));
    });
  }

  protected go(day: string | null) {
    if (day) this.router.navigate(['/days', day]);
  }

  protected step(by: -1 | 1) {
    const d = this.info();
    if (d) this.go(by < 0 ? d.prevDay : d.nextDay);
  }

  protected setIssue(e: { id: number; key: string }) {
    return this.store.setIssue([e.id], e.key);
  }

  protected setComment(e: { id: number; comment: string }) {
    return this.store.setComment([e.id], e.comment);
  }

  protected useTicket(key: string) {
    const ids = this.ticketTargets();
    if (ids.length) return this.store.setIssue(ids, key);
    return undefined;
  }

  /** Each draft without a comment gets its own description as the Tempo comment. */
  protected useDescriptions() {
    return this.store.useDescriptionsAsComments(this.current());
  }

  protected sendQuestion() {
    return this.i18n.t('days.sendQuestion', {
      n: this.sendable().length,
      d: formatDuration(this.sendableSeconds()),
      day: this.title(),
    });
  }

  /** Asks first ("Send 5 worklogs (7h 30m) for Tuesday … to Tempo?"): the OS's own alert where there is one. */
  protected async confirmSend() {
    const yes = await this.alert.confirm(this.i18n.t('days.sendDetail'), {
      title: this.sendQuestion(),
      confirmLabel: this.i18n.t('export.send'),
      cancelLabel: this.i18n.t('status.cancel'),
    });
    if (yes) await this.store.pushDay(this.current());
  }

  /** The day's main action, one step at a time: review the drafts, then send the day to Tempo, then the next day. */
  protected readonly mainAction = computed(() => {
    if (this.drafts().length) return 'review';
    if (this.sendable().length) return 'send';
    return 'next';
  });

  protected async main() {
    switch (this.mainAction()) {
      case 'review':
        return this.store.setStatus(
          this.drafts().map((w) => w.id),
          'reviewed',
        );
      case 'send':
        return this.confirmSend();
      default:
        return this.go(this.next());
    }
  }
}
