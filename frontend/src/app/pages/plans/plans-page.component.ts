import { ChangeDetectionStrategy, Component, computed, inject, input, resource, signal } from '@angular/core';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideArrowUpRight, lucideCalendarClock, lucideRefreshCw, lucideSearchX } from '@ng-icons/lucide';
import { SnCommandBarImports, SnPageCommandsTemplate, SnStatusPage, SnTooltip } from '@spartan-native/desktop-ui';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { API, type Focus, type PlanDayDto, type PlanItemDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { clearFocus, openFocus } from '../../shared/focus';
import { SOURCE_ICON, provideSourceIcons } from '../../shared/source-icons';
import { dayTitle } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { ListFilterComponent } from '../../ui/list-filter/list-filter.component';

/**
 * Tempo plans of the month: per day, each ticket Tempo Planner planned for you (Jira or Tempo adds them when a ticket
 * is assigned or changes status) and each worklog Tempo has that WorkLogger didn't send, next to WorkLogger's own
 * commits, chats and meetings on the same ticket. A day's entries without a ticket can take its planned ticket.
 */
@Component({
  selector: 'wl-plans-page',
  imports: [
    SnTooltip,
    DurationPipe,
    HlmBadgeImports,
    HlmButtonImports,
    ListFilterComponent,
    NgIcon,
    SnCommandBarImports,
    SnPageCommandsTemplate,
    SnStatusPage,
    TPipe,
  ],
  providers: [
    provideSourceIcons(),
    provideIcons({ lucideArrowUpRight, lucideCalendarClock, lucideRefreshCw, lucideSearchX }),
  ],
  templateUrl: './plans-page.component.html',
  styleUrl: './plans-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PlansPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);
  private readonly router = inject(Router);
  protected readonly icon = SOURCE_ICON;

  /** `?day=`: only that day's plans. */
  readonly day = input<string>();
  /** The search box: matched against the ticket, the plan's text and its entries' texts. */
  protected readonly query = signal('');
  protected readonly reading = signal(false);

  /** Read again after every read-back and every change to the month (a ticket given here shows at once). */
  protected readonly plans = resource({
    params: () =>
      this.store.month()
        ? { month: this.store.month(), reads: this.store.tempoReads(), data: this.store.data() }
        : undefined,
    loader: ({ params }) => this.api.plans(params.month),
  });

  protected readonly read = computed(() => this.plans.value()?.read ?? false);
  private readonly all = computed(() => this.plans.value()?.days ?? []);
  protected readonly planned = computed(() => this.plans.value()?.planned ?? 0);
  protected readonly filtered = computed(() => !!this.day() || !!this.query().trim());
  protected readonly days = computed(() => {
    const day = this.day();
    const q = this.query().trim().toLocaleLowerCase();
    return this.all()
      .filter((d) => !day || d.day === day)
      .map((d) => ({ ...d, items: q ? d.items.filter((it) => itemMatches(it, q)) : d.items }))
      .filter((d) => d.items.length > 0)
      .map((d) => ({ ...d, title: dayTitle(d.day, this.i18n.lang()) }));
  });
  protected readonly count = computed(() => this.days().reduce((n, d) => n + d.items.length, 0));
  protected readonly total = computed(() => this.all().reduce((n, d) => n + d.items.length, 0));

  protected async refresh() {
    this.reading.set(true);
    try {
      await this.store.refreshTempo();
    } finally {
      this.reading.set(false);
    }
  }

  /** The day's entries without a ticket go to the planned one. */
  protected async useForUntagged(d: PlanDayDto) {
    if (!d.suggest || !d.untagged.length) return;
    const key = d.suggest;
    await this.store.attempt(() =>
      this.api.setIssue(
        d.untagged.map((e) => e.id),
        key,
      ),
    );
    await this.store.setMonth(this.store.month());
    this.store.status.set(this.i18n.t('plans.used', { n: d.untagged.length, key }));
  }

  protected panes() {
    return this.store.paneIds();
  }

  protected open(f: Focus) {
    return openFocus(this.router, f);
  }

  protected setDay(day: string | null) {
    return this.router.navigate([], { queryParams: { day }, queryParamsHandling: 'merge' });
  }

  protected clearFilter() {
    this.query.set('');
    return clearFocus(this.router);
  }

  protected openProfiles() {
    return this.router.navigate(['settings'], { queryParams: { tab: 'profiles' } });
  }

  protected trackItem(it: PlanItemDto) {
    return it.kind + '|' + it.key;
  }
}

/** `q` is lower-case and trimmed. */
function itemMatches(it: PlanItemDto, q: string) {
  return [it.key, it.description, ...it.entries.map((e) => e.description)].some((v) =>
    v.toLocaleLowerCase().includes(q),
  );
}
