import { ChangeDetectionStrategy, Component, computed, inject, input, resource, signal } from '@angular/core';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideMail, lucideSearchX } from '@ng-icons/lucide';
import { SnStatusPage } from '@spartan-native/desktop-ui';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import { API } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { dayTitle } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { ListFilterComponent } from '../../ui/list-filter/list-filter.component';

/**
 * The mail you sent this month, by day and thread (subject without Re:/Fwd:), as Collect stored it: when you sent
 * each, and the time its draft suggests with its ticket (a key in the subject) or none yet. Local data only; it is
 * read again whenever the month's entries change (after Collect).
 */
@Component({
  selector: 'wl-email-page',
  imports: [
    DurationPipe,
    HlmBadgeImports,
    HlmButtonImports,
    HlmSpinnerImports,
    ListFilterComponent,
    NgIcon,
    SnStatusPage,
    TPipe,
  ],
  providers: [provideIcons({ lucideMail, lucideSearchX })],
  templateUrl: './email-page.component.html',
  // The same layout as the Jira and Confluence feeds: one line per thread, the same columns.
  styleUrl: '../jira-activity/jira-activity-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EmailPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);
  private readonly router = inject(Router);

  /** `?day=`: only that day. */
  readonly day = input<string>();
  /** The search box: matched against the subjects and the ticket. */
  protected readonly query = signal('');

  protected readonly feed = resource({
    params: () => {
      const { from, to } = this.store.range();
      // The entries change after Collect or Rebuild: read again then.
      return from && !this.store.locked() ? { from, to, logs: this.store.worklogs() } : undefined;
    },
    loader: ({ params }) => this.api.sentEmails(params.from, params.to),
  });

  private readonly all = computed(() => (this.feed.hasValue() ? this.feed.value().days : []));
  protected readonly profiles = computed(() => (this.feed.hasValue() ? this.feed.value().profiles : -1));
  protected readonly mails = computed(() => (this.feed.hasValue() ? this.feed.value().mails : 0));
  protected readonly threads = computed(() => (this.feed.hasValue() ? this.feed.value().threads : 0));
  protected readonly error = computed(() => (this.feed.error() ? this.i18n.error(this.feed.error()) : ''));
  protected readonly filtered = computed(() => !!this.day() || !!this.query().trim());
  protected readonly days = computed(() => {
    const day = this.day();
    const q = this.query().trim().toLocaleLowerCase();
    return this.all()
      .filter((d) => !day || d.day === day)
      .map((d) => ({
        ...d,
        threads: q
          ? d.threads.filter((t) => [t.issueKey, ...t.subjects].some((v) => v.toLocaleLowerCase().includes(q)))
          : d.threads,
      }))
      .filter((d) => d.threads.length > 0)
      .map((d) => ({ ...d, title: dayTitle(d.day, this.i18n.lang()) }));
  });
  protected readonly count = computed(() => this.days().reduce((n, d) => n + d.threads.length, 0));

  protected setDay(day: string | null) {
    return this.router.navigate([], { queryParams: { day }, queryParamsHandling: 'merge' });
  }

  protected clearFilter() {
    this.query.set('');
    return this.setDay(null);
  }

  protected openProfiles() {
    return this.router.navigate(['settings'], { queryParams: { tab: 'profiles' } });
  }
}
