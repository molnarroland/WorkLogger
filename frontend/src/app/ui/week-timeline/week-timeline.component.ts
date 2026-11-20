import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import type { DayDto, WorklogDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { NO_TICKET_COLOR, SOURCE_COLOR, TRIAGE_COLOR } from '../../shared/colors';
import { hm, parseDay } from '../../shared/worklogs';

/** rem per hour of the day. */
const HOUR = 3;

interface Block {
  id: number;
  key: string | null;
  text: string;
  range: string;
  top: number;
  height: number;
  color: string;
  missing: boolean;
  short: boolean;
}

/**
 * A week as a calendar (layout E): each day a column, each entry a block where it happened. Weekends show only when
 * something is on them. Entries without a ticket are dashed orange; a past workday with nothing logged gets a slot
 * to open it. Every block and day heading emits `open` with its day. Days and entries come as the backend sends
 * them (WeekDto): totals, states and minutes are theirs; this only places them.
 */
@Component({
  selector: 'wl-week-timeline',
  imports: [DurationPipe, TPipe],
  templateUrl: './week-timeline.component.html',
  styleUrl: './week-timeline.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WeekTimelineComponent {
  private readonly i18n = inject(I18n);
  /** Monday to Sunday. */
  readonly days = input.required<DayDto[]>();
  readonly logs = input.required<WorklogDto[]>();
  readonly today = input('');
  readonly open = output<string>();
  /** An entry's block: its day, with the entry selected. */
  readonly openEntry = output<{ day: string; id: number }>();

  /** First and last hour shown: 8 to 18, stretched to fit the entries. */
  protected readonly hours = computed(() => {
    let from = 8;
    let to = 18;
    for (const w of this.logs()) {
      from = Math.min(from, Math.floor(w.startMinute / 60));
      to = Math.max(to, Math.min(24, Math.ceil((w.startMinute + w.duration / 60) / 60)));
    }
    return Array.from({ length: to - from + 1 }, (_, i) => from + i);
  });
  protected readonly height = computed(() => (this.hours().length - 1) * HOUR);

  protected readonly columns = computed(() => {
    const first = this.hours()[0] * 60;
    const fmt = new Intl.DateTimeFormat(this.i18n.lang(), { weekday: 'short' });
    return this.days()
      .map((d) => {
        const blocks = this.logs()
          .filter((w) => w.day === d.day)
          .map((w): Block => {
            const minutes = w.duration / 60;
            return {
              id: w.id,
              key: w.issueKey,
              text: w.description,
              range: `${hm(w.start)}–${hm(w.end)}`,
              top: ((w.startMinute - first) / 60) * HOUR,
              height: Math.max((minutes / 60) * HOUR, 1.1),
              color: w.needsIssue ? NO_TICKET_COLOR : SOURCE_COLOR[w.source],
              missing: w.needsIssue,
              short: minutes < 45,
            };
          });
        return {
          day: d.day,
          weekday: fmt.format(parseDay(d.day)),
          date: parseDay(d.day).getDate(),
          isToday: d.day === this.today(),
          expected: d.expected,
          logged: d.logged,
          missing: d.missing,
          empty: d.state === 'empty',
          color: TRIAGE_COLOR[d.state],
          blocks,
          weekend: parseDay(d.day).getDay() % 6 === 0,
        };
      })
      .filter((c) => !c.weekend || c.blocks.length > 0 || c.expected > 0);
  });

  /** Where 9:00 sits, for the empty-day slot (a workday's usual start). */
  protected readonly slotTop = computed(() => Math.max(0, 9 - this.hours()[0]) * HOUR);
  protected readonly HOUR = HOUR;
}
