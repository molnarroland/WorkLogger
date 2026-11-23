import { ChangeDetectionStrategy, Component, computed, input, linkedSignal, output } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorCaretLeft, phosphorCaretRight } from '@ng-icons/phosphor-icons/regular';
import { TPipe } from '../../i18n/t.pipe';
import { monthTitle } from '../../shared/month';
import { dayKey, parseDay } from '../../shared/worklogs';
import { HlmButtonImports } from '@spartan-ng/helm/button';

export type PickMode = 'day' | 'week' | 'month';

/** Shifts a month key ("2026-11") by n months. */
function addMonths(month: string, n: number): string {
  const d = new Date(Number(month.slice(0, 4)), Number(month.slice(5, 7)) - 1 + n, 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`;
}

/** The Monday on or before a day. */
function monday(day: string): string {
  const d = parseDay(day);
  d.setDate(d.getDate() - ((d.getDay() + 6) % 7));
  return dayKey(d);
}

/**
 * A small calendar to jump with: a day, a week (any day picks its week, the row is the week), or a month of a year.
 * It emits a day key, or a month key in month mode.
 */
@Component({
  selector: 'wl-period-picker',
  imports: [HlmButtonImports, NgIcon, TPipe],
  providers: [provideIcons({ phosphorCaretLeft, phosphorCaretRight })],
  templateUrl: './period-picker.component.html',
  styleUrl: './period-picker.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PeriodPickerComponent {
  readonly mode = input<PickMode>('day');
  /** The day (day, week) or month (month) on screen. */
  readonly value = input('');
  readonly today = input('');
  readonly lang = input('en');
  readonly pick = output<string>();

  /** The month shown (day, week), or a month of the year shown (month). */
  protected readonly shown = linkedSignal(() => (this.value() || this.today()).slice(0, 7));
  protected readonly title = computed(() =>
    this.mode() === 'month' ? this.shown().slice(0, 4) : monthTitle(this.shown(), this.lang()),
  );
  protected readonly weekdays = computed(() => {
    const fmt = new Intl.DateTimeFormat(this.lang(), { weekday: 'narrow' });
    return Array.from({ length: 7 }, (_, i) => fmt.format(new Date(2024, 0, 1 + i))); // 2024-01-01 is a Monday
  });
  /** Six Monday-first weeks covering the shown month. */
  protected readonly weeks = computed(() => {
    const first = parseDay(monday(this.shown() + '-01'));
    return Array.from({ length: 6 }, (_, w) =>
      Array.from({ length: 7 }, (_, d) => {
        const day = new Date(first);
        day.setDate(first.getDate() + w * 7 + d);
        return { key: dayKey(day), n: day.getDate(), out: dayKey(day).slice(0, 7) !== this.shown() };
      }),
    );
  });
  protected readonly selectedWeek = computed(() =>
    this.mode() === 'week' && this.value() ? monday(this.value()) : '',
  );
  protected readonly months = computed(() => {
    const year = this.shown().slice(0, 4);
    const fmt = new Intl.DateTimeFormat(this.lang(), { month: 'short' });
    return Array.from({ length: 12 }, (_, i) => ({
      key: `${year}-${String(i + 1).padStart(2, '0')}`,
      name: fmt.format(new Date(Number(year), i, 1)),
    }));
  });

  protected step(by: -1 | 1) {
    this.shown.update((m) => addMonths(m, this.mode() === 'month' ? by * 12 : by));
  }

  protected isMonday(day: string) {
    return monday(day) === day;
  }
}
