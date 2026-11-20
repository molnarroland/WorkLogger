import { ChangeDetectionStrategy, Component, input, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideCheck, lucideCopy } from '@ng-icons/lucide';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { TPipe } from '../../i18n/t.pipe';

/** Copies `text` and says "Copied" for a moment. `prominent` is the suggested (accent) button. */
@Component({
  selector: 'wl-copy-button',
  imports: [HlmButtonImports, NgIcon, TPipe],
  providers: [provideIcons({ lucideCheck, lucideCopy })],
  templateUrl: './copy-button.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CopyButtonComponent {
  readonly label = input.required<string>();
  readonly text = input.required<string>();
  /** Longer accessible name when the visible label is short ("Duration"). */
  readonly ariaLabel = input('');
  readonly prominent = input(false);
  readonly small = input(false);
  /** A copy icon instead of the label (which becomes the tooltip); a tick says it was copied. */
  readonly iconOnly = input(false);
  protected readonly copied = signal(false);

  protected async copy() {
    try {
      await navigator.clipboard.writeText(this.text());
    } catch {
      return; // no clipboard access (denied, or the window isn't focused)
    }
    this.copied.set(true);
    setTimeout(() => this.copied.set(false), 1200);
  }
}
