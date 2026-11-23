import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorCircleNotch } from '@ng-icons/phosphor-icons/regular';
import { classes } from '@spartan-ng/helm/utils';

@Component({
  selector: 'hlm-spinner',
  exportAs: 'hlmSpinner',
  imports: [NgIcon],
  providers: [provideIcons({ phosphorCircleNotch })],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    'data-slot': 'spinner',
    role: 'status',
    '[attr.aria-label]': 'ariaLabel()',
  },
  template: ` <ng-icon [name]="icon()" /> `,
})
export class HlmSpinner {
  /**
   * The name of the icon to be used as the spinner.
   * Use provideIcons({ ... }) to register custom icons.
   */
  public readonly icon = input<string>('phosphorCircleNotch');

  /** Aria label for the spinner for accessibility. */
  public readonly ariaLabel = input<string>('Loading', { alias: 'aria-label' });

  constructor() {
    classes(() => 'inline-flex text-[length:--spacing(4)] motion-safe:animate-spin');
  }
}
