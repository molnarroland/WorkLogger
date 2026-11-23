import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  afterRenderEffect,
  input,
  output,
  viewChild,
} from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorX } from '@ng-icons/phosphor-icons/regular';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import type { ActivityLine } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';

/** This session's background activity (already masked by the backend), newest at the bottom. Esc closes it. */
@Component({
  selector: 'wl-activity-console',
  imports: [HlmButtonImports, NgIcon, TPipe],
  providers: [provideIcons({ phosphorX })],
  templateUrl: './activity-console.component.html',
  styleUrl: './activity-console.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { role: 'region', 'aria-labelledby': 'activity-title', '(keydown.escape)': 'closed.emit()' },
})
export class ActivityConsoleComponent {
  readonly lines = input<ActivityLine[]>([]);
  readonly closed = output<void>();
  private readonly log = viewChild.required<ElementRef<HTMLElement>>('log');

  /** Follows the newest line, unless the user scrolled up to read. */
  protected atBottom = true;

  constructor() {
    afterRenderEffect(() => {
      this.lines();
      const el = this.log().nativeElement;
      if (this.atBottom) el.scrollTop = el.scrollHeight;
    });
  }

  protected onScroll(el: HTMLElement) {
    this.atBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 8;
  }

  protected time(t: string) {
    return new Date(t).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hourCycle: 'h23',
    });
  }
}
