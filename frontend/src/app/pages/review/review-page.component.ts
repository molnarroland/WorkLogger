import { ChangeDetectionStrategy, Component, computed, inject, linkedSignal, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideInbox, lucideLock, lucidePlus, lucideTriangleAlert } from '@ng-icons/lucide';
import {
  SnInspectorImports,
  SnMenuImports,
  SnResizable,
  SnStatusPage,
  type SnMenuItem,
} from '@spartan-native/desktop-ui';
import { SnDialogActionsImports, SnSelectableImports } from '@spartan-native/helm';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { HlmTableImports } from '@spartan-ng/helm/table';
import { HlmToggleGroupImports } from '@spartan-ng/helm/toggle-group';
import type { Focus, Status, WorklogDto } from '../../api/api';
import { Router } from '@angular/router';
import { openFocus } from '../../shared/focus';
import { SOURCE_COLOR } from '../../shared/colors';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { dayTitle, hm } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { SOURCE_ICON, provideSourceIcons } from '../../shared/source-icons';
import { EntrySheetsComponent } from '../../shell/entry-sheets/entry-sheets.component';
import { WorklogInspectorComponent } from '../../ui/worklog-inspector/worklog-inspector.component';

export type ReviewFilter = 'noTicket' | 'drafts' | 'locked' | 'all';

/**
 * Every entry of the month by day, in one selectable table: click, Shift/Ctrl-click, arrows and Ctrl/⌘+A select
 * (helm's snSelectable). The selection gets the bulk actions; a single selected entry opens in the inspector.
 * Opens on "No ticket" when something blocks Tempo.
 */
@Component({
  selector: 'wl-review-page',
  imports: [
    SnResizable,
    DurationPipe,
    HlmBadgeImports,
    HlmButtonImports,
    HlmInputImports,
    HlmTableImports,
    HlmToggleGroupImports,
    NgIcon,
    SnDialogActionsImports,
    SnInspectorImports,
    SnMenuImports,
    SnSelectableImports,
    SnStatusPage,
    TPipe,
    EntrySheetsComponent,
    WorklogInspectorComponent,
  ],
  providers: [provideSourceIcons(), provideIcons({ lucideInbox, lucideLock, lucidePlus, lucideTriangleAlert })],
  templateUrl: './review-page.component.html',
  styleUrl: './review-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ReviewPageComponent {
  private readonly router = inject(Router);
  /** Opens where the selected entry came from (the inspector's "Show in …"). */
  protected readonly open = (f: Focus) => openFocus(this.router, f);
  protected readonly store = inject(ShellStore);
  private readonly i18n = inject(I18n);
  protected readonly filters: ReviewFilter[] = ['noTicket', 'drafts', 'locked', 'all'];
  protected readonly icon = SOURCE_ICON;
  protected readonly color = SOURCE_COLOR;
  protected readonly hm = hm;

  protected readonly missing = this.store.needsTicket;
  /** Until the user picks one: "No ticket" if anything needs one. */
  protected readonly filter = linkedSignal<ReviewFilter>(() => (this.missing() > 0 ? 'noTicket' : 'all'));
  protected readonly selection = signal<readonly number[]>([]);
  protected readonly bulkKey = signal('');
  protected readonly bulkComment = signal('');
  /** What was typed; the backend normalises it. */
  protected readonly normalizedBulk = computed(() => this.bulkKey().trim());

  /** The filter's entries under their days, with each day's totals (entries come by start time). */
  protected readonly sections = computed(() => {
    const show = this.matcher(this.filter());
    const rows = new Map<string, WorklogDto[]>();
    for (const w of this.store.worklogs()) if (show(w)) rows.set(w.day, [...(rows.get(w.day) ?? []), w]);
    return [...rows].map(([day, list]) => {
      const info = this.store.days().get(day);
      return {
        day,
        title: dayTitle(day, this.i18n.lang()),
        label: info?.label ?? '',
        expected: info?.expected ?? 0,
        total: info?.logged ?? 0,
        missing: info?.missing ?? 0,
        rows: list,
      };
    });
  });

  /** One selected entry: shown and edited in the inspector. */
  protected readonly inspected = computed(() => {
    const ids = this.selection();
    return ids.length === 1 ? (this.store.worklogs().find((w) => w.id === ids[0]) ?? null) : null;
  });

  protected setFilter(value: unknown) {
    if (typeof value === 'string') this.filter.set(value as ReviewFilter);
  }

  protected async applyIssue() {
    const key = this.normalizedBulk();
    if (!key) return;
    await this.store.setIssue([...this.selection()], key);
    this.bulkKey.set('');
  }

  protected async applyComment() {
    const c = this.bulkComment().trim();
    if (!c) return;
    await this.store.setComment([...this.selection()], c);
    this.bulkComment.set('');
  }

  protected setStatus(ids: readonly number[], status: Status) {
    return this.store.setStatus([...ids], status);
  }

  protected async remove(ids: readonly number[]) {
    await this.store.deleteWorklogs([...ids]);
    this.selection.set([]);
  }

  /** Right-click acts on the selection when the row is part of it, otherwise on that row alone. */
  protected rowMenu(w: WorklogDto): SnMenuItem[] {
    const ids = this.selection().includes(w.id) ? this.selection() : [w.id];
    const removable = ids.every((id) => this.store.worklogs().find((x) => x.id === id)?.canDelete);
    return [
      { label: this.i18n.t('day.markReviewed'), action: () => this.setStatus(ids, 'reviewed') },
      { label: this.i18n.t('status.draft'), action: () => this.setStatus(ids, 'draft') },
      { type: 'separator' },
      ...(ids.length === 1
        ? [
            {
              label: this.i18n.t(w.locked ? 'worklog.unlock' : 'worklog.lock'),
              action: () => this.store.updateWorklog({ ...w, locked: !w.locked }),
            },
          ]
        : []),
      {
        label: this.i18n.t('worklog.delete'),
        accelerator: 'delete',
        enabled: removable,
        action: () => this.remove(ids),
      },
    ];
  }

  private matcher(f: ReviewFilter): (w: WorklogDto) => boolean {
    switch (f) {
      case 'noTicket':
        return (w) => w.needsIssue;
      case 'drafts':
        return (w) => w.status === 'draft';
      case 'locked':
        return (w) => w.locked;
      case 'all':
        return () => true;
    }
  }
}
