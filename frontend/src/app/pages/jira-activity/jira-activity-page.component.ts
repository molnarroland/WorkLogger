import { ChangeDetectionStrategy, Component, computed, inject, input, resource, signal } from '@angular/core';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideActivity, lucideLightbulb, lucidePaperclip, lucideRefreshCw, lucideSearchX } from '@ng-icons/lucide';
import { SnCommandBarImports, SnPageCommandsTemplate, SnStatusPage, SnTooltip } from '@spartan-native/desktop-ui';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import { API, type JiraTicketDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { SOURCE_ICON, provideSourceIcons } from '../../shared/source-icons';
import { dayTitle } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { ListFilterComponent } from '../../ui/list-filter/list-filter.component';

/** Fields with a name in the UI language; the rest show as Jira names them. */
const FIELDS = ['status', 'assignee', 'priority', 'resolution', 'labels', 'sprint', 'summary', 'description'];

/**
 * What you did in Jira this month, like Jira's own activity stream, by day and ticket: the ticket with its status
 * changes at the end of its line and its attachments in their own column, its comments and other changes under
 * it, and the day's entries without a ticket that ran around it, with a button that gives them its key. Read from
 * Jira (a JQL search) when the month changes or on Refresh; nothing is stored.
 */
@Component({
  selector: 'wl-jira-activity-page',
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
  providers: [
    provideSourceIcons(),
    provideIcons({ lucideActivity, lucideLightbulb, lucidePaperclip, lucideRefreshCw, lucideSearchX }),
  ],
  templateUrl: './jira-activity-page.component.html',
  styleUrl: './jira-activity-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class JiraActivityPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);
  private readonly router = inject(Router);
  protected readonly icon = SOURCE_ICON;

  /** `?day=`: only that day. */
  readonly day = input<string>();
  /** The search box: matched against the ticket, its summary, comments, attachments and changes. */
  protected readonly query = signal('');
  /** Refresh (and a key given to entries) reads Jira again for the same month. */
  private readonly reads = signal(0);

  protected readonly feed = resource({
    params: () => {
      const { from, to } = this.store.range();
      return from && !this.store.locked() ? { from, to, reads: this.reads() } : undefined;
    },
    loader: ({ params }) => this.api.jiraActivity(params.from, params.to),
  });

  private readonly all = computed(() => (this.feed.hasValue() ? this.feed.value().days : []));
  protected readonly accounts = computed(() => (this.feed.hasValue() ? this.feed.value().accounts : -1));
  protected readonly entries = computed(() => (this.feed.hasValue() ? this.feed.value().entries : 0));
  protected readonly tickets = computed(() => (this.feed.hasValue() ? this.feed.value().tickets : 0));
  protected readonly error = computed(() => (this.feed.error() ? this.i18n.error(this.feed.error()) : ''));
  protected readonly filtered = computed(() => !!this.day() || !!this.query().trim());
  protected readonly days = computed(() => {
    const day = this.day();
    const q = this.query().trim().toLocaleLowerCase();
    return this.all()
      .filter((d) => !day || d.day === day)
      .map((d) => ({ ...d, tickets: q ? d.tickets.filter((t) => matches(t, q)) : d.tickets }))
      .filter((d) => d.tickets.length > 0)
      .map((d) => ({ ...d, title: dayTitle(d.day, this.i18n.lang()) }));
  });
  protected readonly count = computed(() => this.days().reduce((n, d) => n + d.tickets.length, 0));

  protected refresh() {
    this.reads.update((n) => n + 1);
  }

  /** "To Do → In Progress → Done": where the ticket's status went that day. */
  protected statusChain(t: JiraTicketDto) {
    if (!t.statuses.length) return [];
    return [t.statuses[0].from || this.i18n.t('jira.none'), ...t.statuses.map((s) => s.to || this.i18n.t('jira.none'))];
  }

  /** "Status", or the field as Jira names it. */
  protected fieldLabel(field: string) {
    return FIELDS.includes(field.toLowerCase()) ? this.i18n.t('jira.field.' + field.toLowerCase()) : field;
  }

  protected openIssue(t: JiraTicketDto) {
    return this.store.attempt(() => this.api.openJiraIssue(t.account, t.issueKey));
  }

  /** The suggested entries take the ticket's key; the month and the feed are read again. */
  protected async use(t: JiraTicketDto) {
    const ids = t.suggested.map((e) => e.id);
    if (!ids.length) return;
    await this.store.setIssue(ids, t.issueKey);
    this.store.status.set(this.i18n.t('jira.used', { n: ids.length, key: t.issueKey }));
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

/** `q` is lower-case and trimmed. */
function matches(t: JiraTicketDto, q: string) {
  return [
    t.issueKey,
    t.summary,
    ...t.comments.map((c) => c.text),
    ...t.attachments.map((a) => a.text),
    ...[...t.statuses, ...t.changes].flatMap((c) => [c.field, c.from, c.to]),
  ].some((v) => v.toLocaleLowerCase().includes(q));
}
