import { ChangeDetectionStrategy, Component, computed, inject, input, resource, signal } from '@angular/core';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideBookOpen, lucideLightbulb, lucideRefreshCw, lucideSearchX } from '@ng-icons/lucide';
import { SnCommandBarImports, SnPageCommandsTemplate, SnStatusPage, SnTooltip } from '@spartan-native/desktop-ui';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import { API, type ConfluenceChangeDto, type ConfluencePageDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { SOURCE_ICON, provideSourceIcons } from '../../shared/source-icons';
import { dayTitle } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { ListFilterComponent } from '../../ui/list-filter/list-filter.component';

/**
 * What you did in Confluence this month, by day and page, like the Jira activity page: the page with its space,
 * how often you saved and commented on it, and the time its draft suggests; under it, when you did, and for a page
 * with a ticket key in its title, the day's entries without a ticket that ran around it with a button that gives
 * them the key. Your comments show their text; what each version changed (lines added and removed) is read on
 * request, page by page, since it takes two reads per version. Read from Confluence when the month changes or on
 * Refresh.
 */
@Component({
  selector: 'wl-confluence-activity-page',
  imports: [
    SnTooltip,
    DurationPipe,
    HlmBadgeImports,
    HlmButtonImports,
    HlmSpinnerImports,
    ListFilterComponent,
    NgIcon,
    SnCommandBarImports,
    SnPageCommandsTemplate,
    SnStatusPage,
    TPipe,
  ],
  providers: [provideSourceIcons(), provideIcons({ lucideBookOpen, lucideLightbulb, lucideRefreshCw, lucideSearchX })],
  templateUrl: './confluence-activity-page.component.html',
  // The same layout as the Jira feed (one line per page, the same columns), plus the changes.
  styleUrls: ['../jira-activity/jira-activity-page.component.css', './confluence-activity-page.component.css'],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ConfluenceActivityPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);
  private readonly router = inject(Router);
  protected readonly icon = SOURCE_ICON;

  /** `?day=`: only that day. */
  readonly day = input<string>();
  /** The search box: matched against the page's title, space and ticket. */
  protected readonly query = signal('');
  /** Refresh (and a key given to entries) reads Confluence again for the same month. */
  private readonly reads = signal(0);

  protected readonly feed = resource({
    params: () => {
      const { from, to } = this.store.range();
      return from && !this.store.locked() ? { from, to, reads: this.reads() } : undefined;
    },
    loader: ({ params }) => this.api.confluenceActivity(params.from, params.to),
  });

  private readonly all = computed(() => (this.feed.hasValue() ? this.feed.value().days : []));
  protected readonly profiles = computed(() => (this.feed.hasValue() ? this.feed.value().profiles : -1));
  protected readonly entries = computed(() => (this.feed.hasValue() ? this.feed.value().entries : 0));
  protected readonly pages = computed(() => (this.feed.hasValue() ? this.feed.value().pages : 0));
  protected readonly error = computed(() => (this.feed.error() ? this.i18n.error(this.feed.error()) : ''));
  protected readonly filtered = computed(() => !!this.day() || !!this.query().trim());
  protected readonly days = computed(() => {
    const day = this.day();
    const q = this.query().trim().toLocaleLowerCase();
    return this.all()
      .filter((d) => !day || d.day === day)
      .map((d) => ({
        ...d,
        pages: q
          ? d.pages.filter((p) => [p.title, p.space, p.issueKey].some((v) => v.toLocaleLowerCase().includes(q)))
          : d.pages,
      }))
      .filter((d) => d.pages.length > 0)
      .map((d) => ({ ...d, title: dayTitle(d.day, this.i18n.lang()) }));
  });
  protected readonly count = computed(() => this.days().reduce((n, d) => n + d.pages.length, 0));

  /** What each page-day's versions changed, by `changeKey`, once asked for ('loading' meanwhile). */
  protected readonly changes = signal<Record<string, ConfluenceChangeDto[] | 'loading'>>({});

  protected refresh() {
    this.changes.set({});
    this.reads.update((n) => n + 1);
  }

  protected changeKey(day: string, p: ConfluencePageDto) {
    return `${day}|${p.profile}|${p.pageId}`;
  }

  /** Reads what the page's versions changed that day; a failure shows in the status bar and offers it again. */
  protected async loadChanges(day: string, p: ConfluencePageDto) {
    const key = this.changeKey(day, p);
    this.changes.update((c) => ({ ...c, [key]: 'loading' }));
    const list = await this.store.attempt(() => this.api.confluenceChanges(p.profile, p.kind, p.pageId, day));
    this.changes.update((c) => {
      const { [key]: _, ...rest } = c;
      return list ? { ...rest, [key]: list } : rest;
    });
  }

  protected openPage(p: ConfluencePageDto) {
    return this.store.attempt(() => this.api.openConfluencePage(p.profile, p.pageId));
  }

  /** The suggested entries take the page's key; the month and the feed are read again. */
  protected async use(p: ConfluencePageDto) {
    const ids = p.suggested.map((e) => e.id);
    if (!ids.length || !p.issueKey) return;
    await this.store.setIssue(ids, p.issueKey);
    this.store.status.set(this.i18n.t('jira.used', { n: ids.length, key: p.issueKey }));
    this.refresh();
  }

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
