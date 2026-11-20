import { ChangeDetectionStrategy, Component, computed, inject, input, resource } from '@angular/core';
import { provideIcons } from '@ng-icons/core';
import { Router } from '@angular/router';
import { lucideCircleArrowDown, lucideClock, lucideTrash2, lucideUserCog } from '@ng-icons/lucide';
import { SnCommandBarImports, SnPageCommandsTemplate, SnStatusPage } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { API } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { dayTitle } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { DayStripComponent } from '../../ui/day-strip/day-strip.component';

import { ScrollIntoViewDirective } from '../../ui/scroll-into-view.directive';
/** Work sessions of the month, read from HRmaster: one strip per day with net and reported hours (from Go). */
@Component({
  selector: 'wl-sessions-page',
  imports: [
    ScrollIntoViewDirective,
    SnPageCommandsTemplate,
    DayStripComponent,
    DurationPipe,
    HlmButtonImports,
    SnCommandBarImports,
    SnStatusPage,
    TPipe,
  ],
  providers: [provideIcons({ lucideCircleArrowDown, lucideClock, lucideTrash2, lucideUserCog })],
  templateUrl: './sessions-page.component.html',
  styleUrl: './sessions-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SessionsPageComponent {
  /** `?day=`: the day to point at (a link from an absence entry). */
  readonly day = input<string>();
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);

  protected readonly data = resource({
    // Keyed on the month's data too, so a finished fetch (which re-reads the month) shows here.
    params: () =>
      this.store.month() && this.store.data() ? { month: this.store.month(), data: this.store.data() } : undefined,
    loader: ({ params }) => this.api.sessions(params.month),
  });
  private readonly router = inject(Router);
  /** The HRmaster profiles: every one is read, each day from the one that has it. */
  protected readonly profiles = computed(() =>
    (this.store.settings()?.profiles ?? []).filter((p) => p.kind === 'hrmaster'),
  );
  protected readonly hasUrl = computed(() => this.profiles().some((p) => /^https?:\/\/\S+/.test(p.url)));

  /** The backend's day rows, with their titles in the UI language. */
  protected readonly days = computed(() =>
    (this.data.value()?.days ?? []).map((d) => ({ ...d, title: dayTitle(d.day, this.i18n.lang()) })),
  );

  protected openProfiles() {
    this.router.navigate(['settings'], { queryParams: { tab: 'profiles' } });
  }

  protected async fetch() {
    await this.store.run('fetchHrmaster');
  }

  protected async clear() {
    const { from, to } = this.store.range();
    await this.store.attempt(() => this.api.clearSessions(from, to));
    this.data.reload();
  }
}
