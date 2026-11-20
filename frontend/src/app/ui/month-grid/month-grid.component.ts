import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import type { DayDto, WorklogDto } from '../../api/api';
import { I18n } from '../../i18n/t.pipe';
import { parseDay } from '../../shared/worklogs';
import { DayCardComponent, type DropEvent } from '../day-card/day-card.component';

/**
 * Seven columns, Monday first; the weeks share the height evenly so the last one never runs under the status bar.
 * `weeks` is the month's grid as the backend lays it out (MonthDto.weeks).
 */
@Component({
  selector: 'wl-month-grid',
  imports: [DayCardComponent],
  templateUrl: './month-grid.component.html',
  styleUrl: './month-grid.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MonthGridComponent {
  private readonly i18n = inject(I18n);
  readonly weeks = input.required<string[][]>();
  readonly days = input.required<Map<string, DayDto>>();
  readonly worklogs = input<WorklogDto[]>([]);
  readonly today = input('');
  readonly selected = input<string | null>(null);
  readonly daySelect = output<string>();
  readonly dropped = output<DropEvent>();

  protected readonly logsByDay = computed(() => {
    const by = new Map<string, WorklogDto[]>();
    for (const w of this.worklogs()) by.set(w.day, [...(by.get(w.day) ?? []), w]);
    return by;
  });
  protected readonly weekdays = computed(() => {
    const f = new Intl.DateTimeFormat(this.i18n.lang(), { weekday: 'short' });
    return (this.weeks()[0] ?? []).map((d) => f.format(parseDay(d)));
  });
}
