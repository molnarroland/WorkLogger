import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { SIDE_ICONS, sideIcon } from '../../shell/side-icons';
import { NgTemplateOutlet } from '@angular/common';
import { TPipe } from '../../i18n/t.pipe';
import { SnTooltip } from '@spartan-native/desktop-ui';

/** One source page in the dock: its id (the route), i18n label, icon and how much it has for the day or period. */
export interface DockItem {
  id: string;
  label: string;
  icon: string;
  count: number;
  /** In the glass group at the bar's right end (the pages you finish the month on). */
  end?: boolean;
}

/**
 * The source pages as an icon-only bar under the page, each with a badge of how much it has for the selected day
 * or period. A click opens the page narrowed to that day.
 */
@Component({
  selector: 'wl-source-dock',
  imports: [SnTooltip, NgIcon, NgTemplateOutlet, TPipe],
  providers: [provideIcons(SIDE_ICONS)],
  templateUrl: './source-dock.component.html',
  styleUrl: './source-dock.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SourceDockComponent {
  /** The open page's icon is drawn filled. */
  protected readonly sideIcon = sideIcon;
  readonly items = input.required<DockItem[]>();
  /** The page on screen (its id), shown pressed. */
  readonly active = input('');
  readonly open = output<string>();
  protected readonly start = computed(() => this.items().filter((i) => !i.end));
  protected readonly end = computed(() => this.items().filter((i) => i.end));
}
