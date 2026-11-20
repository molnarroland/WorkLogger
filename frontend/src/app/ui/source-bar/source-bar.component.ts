import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { I18n } from '../../i18n/t.pipe';
import type { BarPart } from '../../shared/colors';
import { formatDuration } from '../../shared/worklogs';

/**
 * A capsule split into coloured parts, left to right. The track is `scale` wide (the day's target) but never
 * shorter than the parts' total, so overtime still fits.
 */
@Component({
  selector: 'wl-source-bar',
  templateUrl: './source-bar.component.html',
  styleUrl: './source-bar.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { role: 'img', '[attr.aria-label]': 'label()', '[style.--bar-height.rem]': 'height()' },
})
export class SourceBarComponent {
  private readonly i18n = inject(I18n);
  readonly parts = input.required<BarPart[]>();
  readonly scale = input(0);
  /** rem */
  readonly height = input(0.3125);

  protected readonly full = computed(() =>
    Math.max(
      this.scale(),
      this.parts().reduce((s, p) => s + p.seconds, 0),
      1,
    ),
  );
  protected readonly label = computed(
    () =>
      this.parts()
        .map((p) => `${this.i18n.t(p.label)} ${formatDuration(p.seconds)}`)
        .join(', ') || formatDuration(0),
  );
}
