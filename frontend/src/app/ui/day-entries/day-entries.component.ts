import { ChangeDetectionStrategy, Component, input, model, output } from '@angular/core';
import { NgIcon } from '@ng-icons/core';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmInputImports } from '@spartan-ng/helm/input';
import type { WorklogDto } from '../../api/api';
import { NO_TICKET_COLOR, SOURCE_COLOR } from '../../shared/colors';
import { TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { hm } from '../../shared/worklogs';
import { SOURCE_ICON, provideSourceIcons } from '../../shared/source-icons';
import { SnTooltip } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';

/**
 * One day's entries, in time order (the Days view). What blocks Tempo is fixed in place: an entry without a ticket
 * gets a ticket field, a draft without a comment a comment field; both commit on Enter or when they lose focus.
 * Clicking (or focusing) anywhere on an entry selects it, for the inspector; clicking its text again deselects it.
 */
@Component({
  selector: 'wl-day-entries',
  imports: [HlmButtonImports, SnTooltip, DurationPipe, HlmBadgeImports, HlmInputImports, NgIcon, TPipe],
  providers: [provideSourceIcons()],
  templateUrl: './day-entries.component.html',
  styleUrl: './day-entries.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DayEntriesComponent {
  readonly logs = input.required<WorklogDto[]>();
  readonly selected = model<number | null>(null);
  readonly issue = output<{ id: number; key: string }>();
  readonly comment = output<{ id: number; comment: string }>();

  protected readonly icon = SOURCE_ICON;
  protected readonly hm = hm;

  protected color(w: WorklogDto) {
    return w.needsIssue ? NO_TICKET_COLOR : SOURCE_COLOR[w.source];
  }

  protected toggle(w: WorklogDto) {
    this.selected.set(this.selected() === w.id ? null : w.id);
  }

  /** Anywhere on the row selects it; the text button keeps its own toggle (toggle()). */
  protected select(w: WorklogDto, e: Event) {
    if (!(e.target as Element).closest('.describe')) this.selected.set(w.id);
  }

  protected setIssue(w: WorklogDto, el: HTMLInputElement) {
    const key = el.value.trim(); // the backend normalises it
    if (key) this.issue.emit({ id: w.id, key });
  }

  protected setComment(w: WorklogDto, el: HTMLInputElement) {
    const comment = el.value.trim();
    if (comment) this.comment.emit({ id: w.id, comment });
  }

  protected commit(e: Event) {
    (e.target as HTMLInputElement).blur();
  }
}
