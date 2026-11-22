import { ChangeDetectionStrategy, Component, computed, inject, input, resource, signal } from '@angular/core';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  lucideArrowUpRight,
  lucideCalendarClock,
  lucideGitCommitHorizontal,
  lucideRefreshCw,
  lucideSearchX,
  lucideTextSearch,
} from '@ng-icons/lucide';
import { SnCommandBarImports, SnPageCommandsTemplate, SnStatusPage, SnTooltip } from '@spartan-native/desktop-ui';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { API, type Focus, type PlanDayDto, type PlanEntryDto, type PlanItemDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { clearFocus, openFocus } from '../../shared/focus';
import { SOURCE_ICON, provideSourceIcons } from '../../shared/source-icons';
import { dayTitle } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { CommitSheetComponent } from '../../shell/commit-sheet/commit-sheet.component';
import { IssuePickerComponent } from '../../shell/issue-picker/issue-picker.component';
import { ListFilterComponent } from '../../ui/list-filter/list-filter.component';

/**
 * Tempo plans of the month: per day, each ticket Tempo Planner planned for you (Jira or Tempo adds them when a ticket
 * is assigned or changes status) and each worklog Tempo has that WorkLogger didn't send, next to WorkLogger's own
 * commits, chats and meetings on the same ticket. A day's entries without a ticket can take its planned ticket, or
 * one chosen from the tickets of this period and before, or found in Jira. A git entry's commits can be opened.
 */
@Component({
  selector: 'wl-plans-page',
  imports: [
    SnTooltip,
    CommitSheetComponent,
    DurationPipe,
    HlmBadgeImports,
    HlmButtonImports,
    IssuePickerComponent,
    ListFilterComponent,
    NgIcon,
    SnCommandBarImports,
    SnPageCommandsTemplate,
    SnStatusPage,
    TPipe,
  ],
  providers: [
    provideSourceIcons(),
    provideIcons({
      lucideArrowUpRight,
      lucideCalendarClock,
      lucideGitCommitHorizontal,
      lucideRefreshCw,
      lucideSearchX,
      lucideTextSearch,
    }),
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
      .map((d) => ({
        ...d,
        items: q ? d.items.filter((it) => itemMatches(it, q)) : d.items,
        untagged: q ? d.untagged.filter((e) => e.description.toLocaleLowerCase().includes(q)) : d.untagged,
      }))
      .filter((d) => d.items.length > 0 || d.untagged.length > 0)
      .map((d) => ({ ...d, title: dayTitle(d.day, this.i18n.lang()) }));
  });
  protected readonly count = computed(() => this.days().reduce((n, d) => n + d.items.length, 0));
  protected readonly total = computed(() => this.all().reduce((n, d) => n + d.items.length, 0));
  /** Something to show: plans, or entries without a ticket. */
  protected readonly any = computed(() => this.all().some((d) => d.items.length || d.untagged.length));

  /** The ticket picker: for which entries of which day. */
  protected readonly picker = signal<{ day: string; ids: number[]; subject: string } | null>(null);
  protected readonly pickerOpen = signal(false);
  /** The commit viewer: which git entry. */
  protected readonly commitsOf = signal<PlanEntryDto | null>(null);
  protected readonly commitsOpen = signal(false);

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

  /** Chooses a ticket for some of the day's entries without one (all of them when `entries` is omitted). */
  protected choose(d: PlanDayDto, entries: PlanEntryDto[] = d.untagged) {
    if (!entries.length) return;
    const subject =
      entries.length === 1 ? entries[0].description : this.i18n.t('plans.untagged', { n: entries.length });
    this.picker.set({ day: d.day, ids: entries.map((e) => e.id), subject });
    this.pickerOpen.set(true);
  }

  protected async picked(key: string) {
    const p = this.picker();
    if (!p) return;
    const ok = await this.store.attempt(async () => {
      await this.api.setIssue(p.ids, key);
      return true;
    });
    if (!ok) return;
    await this.store.setMonth(this.store.month());
    this.store.status.set(this.i18n.t('plans.used', { n: p.ids.length, key }));
  }

  protected showCommits(e: PlanEntryDto) {
    this.commitsOf.set(e);
    this.commitsOpen.set(true);
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
