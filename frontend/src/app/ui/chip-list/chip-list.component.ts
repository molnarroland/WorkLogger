import { ChangeDetectionStrategy, Component, input, model } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideX } from '@ng-icons/lucide';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { TPipe } from '../../i18n/t.pipe';

/** Values as removable badges, with a field to add more (Enter or leaving the field adds; commas split several). */
@Component({
  selector: 'wl-chip-list',
  imports: [HlmBadgeImports, HlmButtonImports, HlmInputImports, NgIcon, TPipe],
  providers: [provideIcons({ lucideX })],
  templateUrl: './chip-list.component.html',
  styleUrl: './chip-list.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ChipListComponent {
  readonly values = model.required<string[]>();
  /** Accessible name of the add field. */
  readonly label = input.required<string>();
  readonly placeholder = input('');
  /** Accent badges (e.g. your own commit emails). */
  readonly highlighted = input(false);

  protected add(field: HTMLInputElement) {
    const next = field.value
      .split(',')
      .map((s) => s.trim())
      .filter((s) => s && !this.values().includes(s));
    field.value = '';
    if (next.length) this.values.set([...this.values(), ...next]);
  }

  protected remove(v: string) {
    this.values.set(this.values().filter((x) => x !== v));
  }
}
