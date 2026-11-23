import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorCalendarDots, phosphorDotsSixVertical, phosphorPlus } from '@ng-icons/phosphor-icons/regular';
import { SnInspectorImports, SnMenuImports, SnResizable, type SnMenuItem } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmDropdownMenuImports } from '@spartan-ng/helm/dropdown-menu';
import type { WorklogDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { monthTitle } from '../../shared/month';
import type { BarPart } from '../../shared/colors';
import { dayTitle, dragPayload, hm } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import type { DropEvent } from '../../ui/day-card/day-card.component';
import { MonthGridComponent } from '../../ui/month-grid/month-grid.component';
import { SOURCE_ICON, provideSourceIcons } from '../../shared/source-icons';
import { SourceLegendComponent } from '../../ui/source-legend/source-legend.component';

const STATES: BarPart[] = [
  { color: 'var(--wl-state-complete)', seconds: 0, label: 'progress.complete' },
  { color: 'var(--wl-state-partial)', seconds: 0, label: 'progress.partial' },
  { color: 'var(--wl-state-empty)', seconds: 0, label: 'progress.empty' },
  { color: 'var(--wl-state-leave)', seconds: 0, label: 'progress.absence' },
  { color: 'var(--wl-state-extra)', seconds: 0, label: 'progress.extra' },
  { color: 'var(--wl-state-idle)', seconds: 0, label: 'progress.idleLegend' },
];
/** The day cards' lines and bar colours, grouped by where the time comes from. */
const LEGEND_GROUPS: { title: string; parts: BarPart[] }[] = [
  {
    title: 'legend.collected',
    parts: [
      { color: 'var(--wl-source-coding)', seconds: 0, label: 'source.git', icon: SOURCE_ICON.git },
      { color: 'var(--wl-source-meeting)', seconds: 0, label: 'source.meeting', icon: SOURCE_ICON.meeting },
      { color: 'var(--wl-source-chat)', seconds: 0, label: 'source.chat', icon: SOURCE_ICON.chat },
    ],
  },
  {
    title: 'legend.added',
    parts: [
      { color: 'var(--wl-source-leave)', seconds: 0, label: 'timesheet.leave', icon: SOURCE_ICON.absence },
      { color: 'var(--wl-source-period)', seconds: 0, label: 'source.period', icon: SOURCE_ICON.period },
      { color: 'var(--wl-source-manual)', seconds: 0, label: 'source.manual', icon: SOURCE_ICON.manual },
    ],
  },
  {
    title: 'legend.attention',
    parts: [{ color: 'var(--wl-needs-ticket)', seconds: 0, label: 'source.noTicket', icon: 'phosphorWarning' }],
  },
];

/** Tempo-style month grid: every day shows what is logged against what it should be. A day opens in the inspector. */
@Component({
  selector: 'wl-timesheet-page',
  imports: [
    SnResizable,
    DurationPipe,
    HlmButtonImports,
    HlmDropdownMenuImports,
    MonthGridComponent,
    NgIcon,
    SnInspectorImports,
    SnMenuImports,
    SourceLegendComponent,
    TPipe,
  ],
  providers: [provideSourceIcons(), provideIcons({ phosphorCalendarDots, phosphorDotsSixVertical, phosphorPlus })],
  templateUrl: './timesheet-page.component.html',
  styleUrl: './timesheet-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TimesheetPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly i18n = inject(I18n);
  protected readonly selected = signal<string | null>(null);
  protected readonly states = STATES;
  protected readonly legendGroups = LEGEND_GROUPS;
  protected readonly icon = SOURCE_ICON;
  protected readonly hm = hm;

  protected readonly title = computed(() => monthTitle(this.store.month(), this.i18n.lang()));
  protected readonly logged = computed(() => this.store.summary()?.logged ?? 0);
  protected readonly expected = computed(() => this.store.summary()?.expected ?? 0);
  protected readonly gap = computed(() => this.store.summary()?.short ?? 0);
  protected readonly missing = this.store.needsTicket;
  protected readonly selectedDay = computed(() => this.store.days().get(this.selected() ?? '') ?? null);
  protected readonly selectedTitle = computed(() => this.dayTitle(this.selected() ?? ''));
  /** The open day's entries (the backend sends them by start time). */
  protected readonly selectedLogs = computed(() => this.store.worklogs().filter((w) => w.day === this.selected()));
  /** Days an entry can move to (the backend's move targets), other than the open one. */
  protected readonly moveTargets = computed(() =>
    [...this.store.days().values()].filter((d) => d.moveTarget && d.day !== this.selected()),
  );

  protected toggle(day: string) {
    this.selected.update((s) => (s === day ? null : day));
  }

  protected move(e: DropEvent) {
    return this.store.moveWorklogs(e.ids, e.day);
  }

  protected markReviewed() {
    return this.store.setStatus(
      this.selectedLogs().map((w) => w.id),
      'reviewed',
    );
  }

  protected dayTitle(day: string) {
    return day ? dayTitle(day, this.i18n.lang()) : '';
  }

  protected onDragStart(e: DragEvent, w: WorklogDto) {
    e.dataTransfer?.setData('text/plain', dragPayload([w]));
  }

  /** Right-click on an entry: the same moves as its Move to… menu. */
  protected entryMenu(w: WorklogDto): SnMenuItem[] {
    if (!w.canMove) return [];
    return [
      {
        label: this.i18n.t('timesheet.moveTo'),
        submenu: this.moveTargets().map((t) => ({
          label: this.dayTitle(t.day),
          action: () => this.move({ ids: [w.id], day: t.day }),
        })),
      },
    ];
  }
}
