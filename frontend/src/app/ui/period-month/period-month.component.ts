import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import type { DayDto, ProjectPeriod } from '../../api/api';
import { I18n } from '../../i18n/t.pipe';
import { monthTitle } from '../../shared/month';
import { periodColor } from '../../shared/colors';
import { parseDay } from '../../shared/worklogs';

/**
 * The month with each period's days in its colour, the same colours as the period rows. The grid and which period
 * books each day come from the backend (MonthDto.weeks, DayDto.periodIndex); `periods` only names them.
 */
@Component({
  selector: 'wl-period-month',
  templateUrl: './period-month.component.html',
  styleUrl: './period-month.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PeriodMonthComponent {
  private readonly i18n = inject(I18n);
  readonly month = input.required<string>();
  readonly weeks = input.required<string[][]>();
  readonly days = input.required<ReadonlyMap<string, DayDto>>();
  readonly periods = input<ProjectPeriod[]>([]);

  protected readonly title = computed(() => monthTitle(this.month(), this.i18n.lang()));
  protected readonly weekdays = computed(() => {
    const f = new Intl.DateTimeFormat(this.i18n.lang(), { weekday: 'narrow' });
    return (this.weeks()[0] ?? []).map((d) => f.format(parseDay(d)));
  });
  protected readonly cells = computed(() =>
    this.weeks()
      .flat()
      .map((day) => {
        const i = this.days().get(day)?.periodIndex ?? -1;
        const p = this.periods()[i];
        return {
          day,
          number: Number(day.slice(8)),
          inMonth: this.days().has(day),
          color: p ? periodColor(i) : null,
          label: p ? `${p.title || p.issueKey} · ${p.issueKey}` : '',
        };
      }),
  );
}
