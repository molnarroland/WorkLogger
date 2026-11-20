import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideBadgeCheck, lucideClipboardList } from '@ng-icons/lucide';
import { SnStatusPage, SnTooltip } from '@spartan-native/desktop-ui';
import { SnSelectableImports } from '@spartan-native/helm';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmCheckboxImports } from '@spartan-ng/helm/checkbox';
import { HlmTableImports } from '@spartan-ng/helm/table';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import type { TicketGroupDto } from '../../api/api';
import { dayTitle } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { CopyButtonComponent } from '../../ui/copy-button/copy-button.component';
import { SourceBarComponent } from '../../ui/source-bar/source-bar.component';

/** The list's key for the "No issue" group (selection keys can't be null). */
const NO_ISSUE = '∅';

/** Log in Tempo by hand: pick a ticket, copy each day's duration and text into Tempo, tick it off. */
@Component({
  selector: 'wl-by-ticket-page',
  imports: [
    SnTooltip,
    CopyButtonComponent,
    DurationPipe,
    HlmBadgeImports,
    HlmButtonImports,
    HlmCheckboxImports,
    HlmTableImports,
    NgIcon,
    SnSelectableImports,
    SnStatusPage,
    SourceBarComponent,
    TPipe,
  ],
  providers: [provideIcons({ lucideBadgeCheck, lucideClipboardList })],
  templateUrl: './by-ticket-page.component.html',
  styleUrl: './by-ticket-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ByTicketPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly i18n = inject(I18n);
  private readonly router = inject(Router);
  protected readonly chosen = signal<readonly string[]>([]);

  /** The picked ticket, else the first real ticket (not "No issue"), else whatever there is. */
  protected readonly current = computed(() => {
    const groups = this.store.tickets();
    const picked = this.chosen()[0];
    return (
      groups.find((g) => picked !== undefined && this.keyOf(g) === picked) ??
      groups.find((g) => g.key !== null) ??
      groups[0] ??
      null
    );
  });
  /** The list always shows the ticket on the right as selected. */
  protected readonly selection = computed<readonly string[]>(() => {
    const g = this.current();
    return g ? [this.keyOf(g)] : [];
  });
  protected readonly text = computed(() => this.current()?.text ?? '');

  protected keyOf(g: TicketGroupDto) {
    return g.key ?? NO_ISSUE;
  }

  protected progress(g: TicketGroupDto) {
    return [{ color: 'var(--wl-state-complete)', seconds: g.enteredDays, label: 'ticket.inTempo' }];
  }

  protected title(day: string) {
    return dayTitle(day, this.i18n.lang());
  }

  protected fix() {
    this.router.navigate(['review']);
  }
}
