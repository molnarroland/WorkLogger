import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorWarning } from '@ng-icons/phosphor-icons/regular';
import { TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import type { BarPart } from '../../shared/colors';
import { provideSourceIcons } from '../../shared/source-icons';

/**
 * Swatch + name (+ duration when `showTime`) for each bar part, with the part's icon when it has one. `round` draws
 * dots (day states) instead of squares; `title` names the group.
 */
@Component({
  selector: 'wl-source-legend',
  imports: [DurationPipe, NgIcon, TPipe],
  providers: [provideSourceIcons(), provideIcons({ phosphorWarning })],
  templateUrl: './source-legend.component.html',
  styleUrl: './source-legend.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SourceLegendComponent {
  readonly parts = input.required<BarPart[]>();
  readonly showTime = input(true);
  readonly round = input(false);
  /** i18n key of the group's heading; none = no heading. */
  readonly title = input('');
}
