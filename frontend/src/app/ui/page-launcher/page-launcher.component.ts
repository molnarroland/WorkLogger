import { ChangeDetectionStrategy, Component, ElementRef, inject, input, output } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucidePin } from '@ng-icons/lucide';
import { TPipe } from '../../i18n/t.pipe';
import type { Pane, Step } from '../../shell/panes';
import { SnTooltip } from '@spartan-native/desktop-ui';

/**
 * "All pages": every page as a tile, by workflow step, each with a pin that puts it in the side panel. Meant as a
 * native popover (`<wl-page-launcher popover>`); opening a page closes it. The page icons come from the host.
 */
@Component({
  selector: 'wl-page-launcher',
  imports: [SnTooltip, NgIcon, TPipe],
  providers: [provideIcons({ lucidePin })],
  templateUrl: './page-launcher.component.html',
  styleUrl: './page-launcher.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PageLauncherComponent {
  readonly groups = input<{ step: Step; panes: Pane[] }[]>([]);
  /** The pinned pages' ids. */
  readonly pinned = input<readonly string[]>([]);
  /** The open page's id. */
  readonly current = input('');
  readonly open = output<string>();
  readonly togglePin = output<string>();
  private readonly el: HTMLElement = inject(ElementRef<HTMLElement>).nativeElement;

  protected go(id: string) {
    if (this.el.matches(':popover-open')) this.el.hidePopover();
    this.open.emit(id);
  }
}
