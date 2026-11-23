import { ChangeDetectionStrategy, Component, computed, input, model } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorX } from '@ng-icons/phosphor-icons/regular';
import { SnSearchField } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { TPipe } from '../../i18n/t.pipe';

/**
 * A list's filter line: a search box and a day of the month, with a button that clears both. The page decides
 * what the text matches and what the day means; this only holds the two values.
 */
@Component({
  selector: 'wl-list-filter',
  imports: [HlmButtonImports, HlmInputImports, NgIcon, SnSearchField, TPipe],
  providers: [provideIcons({ phosphorX })],
  templateUrl: './list-filter.component.html',
  styleUrl: './list-filter.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ListFilterComponent {
  /** What to look for; '' for everything. */
  readonly query = model('');
  /** "YYYY-MM-DD", or null for every day. */
  readonly day = model<string | null>(null);
  /** The days the picker offers ("YYYY-MM-DD"): the month on screen. */
  readonly min = input('');
  readonly max = input('');
  /** The search box's placeholder and accessible name. */
  readonly placeholder = input.required<string>();

  protected readonly active = computed(() => !!this.query() || !!this.day());

  protected setDay(e: Event) {
    this.day.set((e.target as HTMLInputElement).value || null);
  }

  protected clear() {
    this.query.set('');
    this.day.set(null);
  }
}
