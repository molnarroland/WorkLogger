import { ChangeDetectionStrategy, Component, computed, inject, input, output, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideTriangleAlert } from '@ng-icons/lucide';
import type { DayDto, WorklogDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { PROGRESS_COLOR, toBarParts } from '../../shared/colors';
import { dayTitle, dragIds, dragPayload, formatDuration } from '../../shared/worklogs';
import { SOURCE_ICON, provideSourceIcons } from '../../shared/source-icons';
import { SourceBarComponent } from '../source-bar/source-bar.component';

export interface DropEvent {
  ids: number[];
  day: string;
}

/**
 * The Timesheet's day: number, up to three "source … duration" lines that can be dragged to another day, the
 * source bar and "logged / expected", tinted by the day's progress. Totals and states come with `info` (from the
 * backend); `logs` are the day's entries, for dragging. The whole card is one button for the keyboard and screen
 * readers. `info` is missing for the neighbouring months' days, which show only their number.
 */
@Component({
  selector: 'wl-day-card',
  imports: [DurationPipe, NgIcon, SourceBarComponent, TPipe],
  providers: [provideSourceIcons(), provideIcons({ lucideTriangleAlert })],
  templateUrl: './day-card.component.html',
  styleUrl: './day-card.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '[class.outside]': '!info()',
    '[class.selected]': 'selected()',
    '[class.drop]': 'dropActive() || dropTarget()',
    '[style.--state]': 'stateColor()',
    '(dragover)': 'onDragOver($event)',
    '(dragleave)': 'dropActive.set(false)',
    '(drop)': 'onDrop($event)',
  },
})
export class DayCardComponent {
  private readonly i18n = inject(I18n);
  readonly day = input.required<string>();
  readonly info = input<DayDto>();
  readonly logs = input<WorklogDto[]>([]);
  /** Marks the day number. */
  readonly today = input('');
  readonly selected = input(false);
  /** Forces the drop-target look (stories); dragging over a day that takes work sets it too. */
  readonly dropTarget = input(false);
  readonly daySelect = output<string>();
  readonly dropped = output<DropEvent>();

  protected readonly icon = SOURCE_ICON;
  protected readonly dropActive = signal(false);
  protected readonly dayNumber = computed(() => Number(this.day().slice(8)));
  protected readonly isToday = computed(() => this.day() === this.today());
  protected readonly logged = computed(() => this.info()?.logged ?? 0);
  protected readonly missing = computed(() => (this.info()?.missing ?? 0) > 0);
  protected readonly sources = computed(() => (this.info()?.sources ?? []).slice(0, 3));
  protected readonly parts = computed(() => toBarParts(this.info()?.parts ?? []));
  protected readonly progress = computed(() => this.info()?.progress ?? 'free');
  protected readonly stateColor = computed(() => (this.info() ? PROGRESS_COLOR[this.progress()] : null));
  /** HR's label on days that aren't plain workdays ("Szabadság"). */
  protected readonly hrLabel = computed(() => {
    const d = this.info();
    return d && (d.absence || d.expected === 0) ? d.label : '';
  });
  protected readonly ariaLabel = computed(() =>
    this.i18n.t('timesheet.dayLabel', {
      date: dayTitle(this.day(), this.i18n.lang()),
      logged: formatDuration(this.logged()),
      expected: formatDuration(this.info()?.expected ?? 0),
      state: this.i18n.t('progress.' + this.progress()),
    }),
  );

  protected onDragStart(e: DragEvent, source: string) {
    e.dataTransfer?.setData('text/plain', dragPayload(this.logs().filter((w) => w.source === source)));
  }

  protected onDragOver(e: DragEvent) {
    if (!this.info()?.acceptsWork) return;
    e.preventDefault(); // allows the drop
    this.dropActive.set(true);
  }

  protected onDrop(e: DragEvent) {
    e.preventDefault();
    this.dropActive.set(false);
    const ids = dragIds(e.dataTransfer?.getData('text/plain') ?? '');
    if (ids.length && this.info()?.acceptsWork) this.dropped.emit({ ids, day: this.day() });
  }
}
