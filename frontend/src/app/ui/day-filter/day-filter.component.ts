import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorX } from '@ng-icons/phosphor-icons/regular';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { dayTitle } from '../../shared/worklogs';
import { HlmButtonImports } from '@spartan-ng/helm/button';

/** "Only Tue, Oct 27 ✕": a page narrowed to one day by a link from elsewhere; the button shows everything again. */
@Component({
  selector: 'wl-day-filter',
  imports: [HlmButtonImports, NgIcon, TPipe],
  providers: [provideIcons({ phosphorX })],
  templateUrl: './day-filter.component.html',
  styleUrl: './day-filter.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DayFilterComponent {
  private readonly i18n = inject(I18n);
  /** "YYYY-MM-DD" */
  readonly day = input.required<string>();
  readonly cleared = output<void>();
  protected readonly title = computed(() => dayTitle(this.day(), this.i18n.lang()));
}
