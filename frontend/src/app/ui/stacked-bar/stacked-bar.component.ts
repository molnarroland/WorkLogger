import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { I18n } from '../../i18n/t.pipe';
import type { BarPart } from '../../shared/colors';
import { formatDuration } from '../../shared/worklogs';

export interface StackedColumn {
  label: string;
  parts: BarPart[];
}

/** Columns of stacked hours (Totals). Plain CSS: only stacked bars are needed, so no chart library. */
@Component({
  selector: 'wl-stacked-bar',
  templateUrl: './stacked-bar.component.html',
  styleUrl: './stacked-bar.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { role: 'img', '[attr.aria-label]': 'summary()' },
})
export class StackedBarComponent {
  private readonly i18n = inject(I18n);
  readonly columns = input.required<StackedColumn[]>();

  /** Top of the axis in hours, rounded up to a step. */
  protected readonly top = computed(() => {
    const max = Math.max(1, ...this.columns().map((c) => c.parts.reduce((s, p) => s + p.seconds, 0) / 3600));
    const step = this.step(max);
    return Math.ceil(max / step) * step;
  });
  protected readonly ticks = computed(() => {
    const step = this.step(this.top());
    return Array.from({ length: this.top() / step + 1 }, (_, i) => i * step);
  });
  protected readonly summary = computed(
    () =>
      this.columns()
        .map((c) => `${c.label}: ${this.total(c)}`)
        .join('; ') || formatDuration(0),
  );

  protected total(c: StackedColumn) {
    const seconds = c.parts.reduce((s, p) => s + p.seconds, 0);
    return `${formatDuration(seconds)} (${c.parts.map((p) => `${this.i18n.t(p.label)} ${formatDuration(p.seconds)}`).join(', ')})`;
  }

  private step(hours: number) {
    return hours <= 4 ? 1 : hours <= 12 ? 2 : hours <= 40 ? 10 : 20;
  }
}
