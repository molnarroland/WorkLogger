import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  input,
  resource,
  untracked,
} from '@angular/core';
import { Router } from '@angular/router';
import { provideIcons } from '@ng-icons/core';
import { lucideMessagesSquare, lucideSend } from '@ng-icons/lucide';
import { SnCommandBarImports, SnPageCommandsTemplate } from '@spartan-native/desktop-ui';
import { API } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { parseDay } from '../../shared/worklogs';
import { ShellStore, providePageHeader } from '../../state/shell.store';
import { WeekTimelineComponent } from '../../ui/week-timeline/week-timeline.component';

const DAY_KEY = /^\d{4}-\d{2}-\d{2}$/;

/**
 * A week as a calendar (layout E), as the backend computes it (Api.week), across a month end when it falls on one.
 * `/week/2026-10-27` shows the week holding that day; `/week` the current week, or the month's first. Clicking a
 * day or an entry opens it in the Days view.
 */
@Component({
  selector: 'wl-week-page',
  imports: [SnPageCommandsTemplate, SnCommandBarImports, TPipe, WeekTimelineComponent],
  providers: [provideIcons({ lucideMessagesSquare, lucideSend })],
  templateUrl: './week-page.component.html',
  styleUrl: './week-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WeekPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly router = inject(Router);
  private readonly i18n = inject(I18n);

  readonly day = input<string>();

  protected readonly anchor = computed(() => {
    const day = this.day();
    if (day && DAY_KEY.test(day)) return day;
    return this.store.data()?.anchorDay ?? '';
  });
  /** Re-read whenever the month is (an edit or a collect changes what the week shows). */
  protected readonly week = resource({
    params: () => (this.anchor() && this.store.data() ? { day: this.anchor(), month: this.store.data() } : undefined),
    loader: ({ params }) => this.api.week(params.day),
  });
  protected readonly days = computed(() => this.week.value()?.days ?? []);
  protected readonly logs = computed(() => this.week.value()?.worklogs ?? []);
  protected readonly logged = computed(() => this.week.value()?.logged ?? 0);
  protected readonly expected = computed(() => this.week.value()?.expected ?? 0);
  protected readonly title = computed(() => {
    const days = this.days();
    if (!days.length) return '';
    const fmt = new Intl.DateTimeFormat(this.i18n.lang(), { month: 'short', day: 'numeric' });
    return fmt.formatRange(parseDay(days[0].day), parseDay(days[6].day));
  });

  constructor() {
    // The week and its hours go to the header line's centre (instead of the month's).
    const heading = computed(() =>
      this.title() ? { title: this.title(), logged: this.logged(), expected: this.expected() } : null,
    );
    providePageHeader(this.store, inject(DestroyRef), {
      previous: 'week.previous',
      next: 'week.next',
      step: (by) => this.step(by),
      stepDays: 7,
      today: {
        label: 'week.this',
        icon: 'lucideCalendarRange',
        go: () => this.router.navigate(['/week', this.store.today()]),
        current: computed(() => this.days().some((d) => d.day === this.store.today())),
      },
      pick: {
        label: 'week.pick',
        mode: 'week',
        value: computed(() => this.days()[0]?.day ?? ''),
        go: (day) => this.router.navigate(['/week', day]),
      },
      heading,
    });
    // Moving to a week of another month brings that month in (the week's month is its Thursday's, as ISO weeks do).
    effect(() => {
      const month = this.week.value()?.month;
      if (month && this.day() && month !== untracked(this.store.month)) this.store.setMonth(month);
    });
  }

  protected step(by: -1 | 1) {
    const w = this.week.value();
    if (w) return this.router.navigate(['/week', by < 0 ? w.prev : w.next]);
    return undefined;
  }

  protected open(day: string, id?: number) {
    return this.router.navigate(['/days', day], { queryParams: { focus: id ?? null } });
  }
}
