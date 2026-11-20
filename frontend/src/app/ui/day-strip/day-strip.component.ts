import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import type { SessionDto } from '../../api/api';
import { I18n } from '../../i18n/t.pipe';
import { hm } from '../../shared/worklogs';

/** A day's sessions on a 07:00–19:00 axis (stretched when a session falls outside): work green, breaks grey. */
@Component({
  selector: 'wl-day-strip',
  templateUrl: './day-strip.component.html',
  styleUrl: './day-strip.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { role: 'img', '[attr.aria-label]': 'label()' },
})
export class DayStripComponent {
  private readonly i18n = inject(I18n);
  readonly sessions = input.required<SessionDto[]>();

  /** The hours shown, from the backend's minutes of the day. */
  private readonly hours = computed(() => {
    const all = this.sessions().flatMap((x) => [x.startMinute / 60, x.endMinute / 60]);
    if (!all.length) return { lo: 7, hi: 19 };
    return { lo: Math.min(7, Math.floor(Math.min(...all))), hi: Math.max(19, Math.ceil(Math.max(...all))) };
  });

  /** Work first, breaks on top. */
  protected readonly bars = computed(() => {
    const { lo, hi } = this.hours();
    const x = (minute: number) => ((minute / 60 - lo) / (hi - lo)) * 100;
    return [...this.sessions()]
      .sort((a, b) => (a.kind === 'work' ? 0 : 1) - (b.kind === 'work' ? 0 : 1))
      .map((s) => ({
        left: x(s.startMinute),
        width: Math.max(0.5, x(s.endMinute) - x(s.startMinute)),
        work: s.kind === 'work',
      }));
  });

  protected readonly ticks = computed(() => {
    const { lo, hi } = this.hours();
    const out: number[] = [];
    for (let h = lo; h <= hi; h += 2) out.push(h);
    return out;
  });

  protected readonly label = computed(
    () =>
      this.sessions()
        .map((s) =>
          this.i18n.t(s.kind === 'work' ? 'sessions.work' : 'sessions.break', { from: hm(s.start), to: hm(s.end) }),
        )
        .join(', ') || this.i18n.t('sessions.none'),
  );
}
