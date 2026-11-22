import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  model,
  output,
  resource,
  signal,
  untracked,
} from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideArrowUpRight, lucideSearch } from '@ng-icons/lucide';
import { SnModalSheet, SnSearchField, SnTooltip } from '@spartan-native/desktop-ui';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import { HlmToggleGroupImports } from '@spartan-ng/helm/toggle-group';
import { API, type IssueOptionDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { dayTitle } from '../../shared/worklogs';

/** PROJ-123, as typed (any case). */
const KEY = /^[A-Za-z][A-Za-z0-9_]*-\d+$/;

/**
 * Chooses a ticket for entries of a day: the tickets planned for it, used that day, the project periods' and those
 * used in this period or an earlier one (filtered as you type); and a Jira search for issues created within one
 * or two weeks of the day, for a ticket WorkLogger hasn't seen yet. Choosing one emits its key and closes.
 */
@Component({
  selector: 'wl-issue-picker',
  imports: [
    HlmBadgeImports,
    HlmButtonImports,
    HlmSpinnerImports,
    HlmToggleGroupImports,
    NgIcon,
    SnModalSheet,
    SnSearchField,
    SnTooltip,
    TPipe,
  ],
  providers: [provideIcons({ lucideArrowUpRight, lucideSearch })],
  templateUrl: './issue-picker.component.html',
  styleUrl: './issue-picker.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IssuePickerComponent {
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);

  readonly open = model(false);
  /** The entries' day ("YYYY-MM-DD"): what "around" means for the Jira search. */
  readonly day = input.required<string>();
  /** What the ticket is for, under the title ("3 entries", the entry's description). */
  readonly subject = input('');
  /** The chosen ticket's key. */
  readonly picked = output<string>();

  protected readonly query = signal('');
  protected readonly weeks = signal<1 | 2>(1);

  protected readonly options = resource({
    params: () => (this.open() ? { day: this.day() } : undefined),
    loader: ({ params }) => this.api.issueOptions(params.day),
  });

  /** The Jira search: what it found for which text and window, or why it failed. */
  protected readonly jira = signal<{ text: string; weeks: number; found: IssueOptionDto[] } | null>(null);
  protected readonly jiraError = signal('');
  protected readonly searching = signal(false);

  protected readonly dayLabel = computed(() => dayTitle(this.day(), this.i18n.lang()));
  protected readonly local = computed(() => {
    const q = this.query().trim().toLocaleLowerCase();
    const all = this.options.value() ?? [];
    return q ? all.filter((o) => matches(o, q)) : all;
  });
  /** A full key typed that no list has: offered as it is. */
  protected readonly typedKey = computed(() => {
    const q = this.query().trim().toUpperCase();
    if (!KEY.test(q)) return null;
    const known = [...(this.options.value() ?? []), ...(this.jira()?.found ?? [])].some((o) => o.key === q);
    return known ? null : q;
  });
  /** The Jira results still match the box and the window (else they are stale and searching again is offered). */
  protected readonly jiraCurrent = computed(() => {
    const j = this.jira();
    return !!j && j.text === this.query().trim() && j.weeks === this.weeks();
  });
  protected readonly window = computed(() => ({
    from: this.shift(-7 * this.weeks()),
    to: this.shift(7 * this.weeks()),
  }));

  constructor() {
    // Each opening starts afresh; with nothing to offer from WorkLogger, Jira is asked straight away.
    effect(() => {
      if (!this.open()) return;
      untracked(() => {
        this.query.set('');
        this.jira.set(null);
        this.jiraError.set('');
      });
    });
    effect(() => {
      const opts = this.options.value();
      if (this.open() && opts && !opts.length) untracked(() => void this.searchJira());
    });
  }

  protected setWeeks(v: unknown) {
    if (v === 1 || v === 2 || v === '1' || v === '2') this.weeks.set(Number(v) as 1 | 2);
  }

  protected async searchJira() {
    if (this.searching()) return;
    const text = this.query().trim();
    const weeks = this.weeks();
    this.searching.set(true);
    this.jiraError.set('');
    try {
      this.jira.set({ text, weeks, found: await this.api.searchJiraIssues(this.day(), text, weeks) });
    } catch (e) {
      this.jira.set(null);
      this.jiraError.set(this.i18n.error(e));
    } finally {
      this.searching.set(false);
    }
  }

  protected pick(key: string) {
    this.picked.emit(key);
    this.open.set(false);
  }

  protected async openInJira(o: IssueOptionDto, event: Event) {
    event.stopPropagation();
    try {
      await this.api.openJiraIssue(o.account, o.key);
    } catch (e) {
      this.jiraError.set(this.i18n.error(e));
    }
  }

  /** "Mar 4": the year only matters for the long titles. */
  protected short(day: string) {
    if (!day) return '';
    const [y, m, d] = day.split('-').map(Number);
    return new Intl.DateTimeFormat(this.i18n.lang(), { month: 'short', day: 'numeric' }).format(new Date(y, m - 1, d));
  }

  private shift(days: number) {
    const [y, m, d] = this.day().split('-').map(Number);
    const t = new Date(y, m - 1, d + days);
    return this.short(`${t.getFullYear()}-${t.getMonth() + 1}-${t.getDate()}`);
  }
}

/** `q` is lower-case and trimmed: every word in the key, the summary or the description. */
function matches(o: IssueOptionDto, q: string) {
  const text = `${o.key} ${o.summary} ${o.description}`.toLocaleLowerCase();
  return q.split(/\s+/).every((w) => text.includes(w));
}
