import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  phosphorCalendarX,
  phosphorCheckCircle,
  phosphorListChecks,
  phosphorPaperPlaneTilt,
  phosphorTag,
} from '@ng-icons/phosphor-icons/regular';
import { SnStatusPage } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import type { TodoDto, TodoKind } from '../../api/api';
import { TRIAGE_COLOR, toBarParts } from '../../shared/colors';
import { monthTitle } from '../../shared/month';
import { dayNote, dayTitle, parseDay } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { SourceBarComponent } from '../../ui/source-bar/source-bar.component';
import { SourceLegendComponent } from '../../ui/source-legend/source-legend.component';

const TODO_ICON: Record<TodoKind, string> = {
  fillDays: 'phosphorCalendarX',
  addTickets: 'phosphorTag',
  reviewDrafts: 'phosphorListChecks',
  logInTempo: 'phosphorPaperPlaneTilt',
};

/**
 * The month at a glance, where the window opens: how much is logged and where it came from, how much is in Tempo,
 * and "What's left": the backend's to-dos for the month, each with the button that does it.
 */
@Component({
  selector: 'wl-overview-page',
  imports: [
    DurationPipe,
    HlmButtonImports,
    NgIcon,
    RouterLink,
    SnStatusPage,
    SourceBarComponent,
    SourceLegendComponent,
    TPipe,
  ],
  providers: [
    provideIcons({ phosphorCalendarX, phosphorCheckCircle, phosphorListChecks, phosphorPaperPlaneTilt, phosphorTag }),
  ],
  templateUrl: './overview-page.component.html',
  styleUrl: './overview-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OverviewPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly router = inject(Router);
  private readonly i18n = inject(I18n);
  protected readonly icon = TODO_ICON;

  protected readonly summary = this.store.summary;
  /** The month's days in weeks Tempo has in full (their source data can go). */
  protected readonly settled = computed(() => this.store.data()?.settled.length ?? 0);
  protected readonly parts = computed(() => toBarParts(this.summary()?.parts ?? []));
  protected readonly todos = this.store.todos;
  protected readonly monthName = computed(() => monthTitle(this.store.month(), this.i18n.lang()));
  protected readonly empty = computed(() => this.store.worklogs().length === 0);
  /** Next up: the first day that needs work (the side panel's "Needs attention"), with what it needs. */
  protected readonly next = computed(() => {
    const d = this.store.attention()[0];
    if (!d) return null;
    return {
      day: d.day,
      color: TRIAGE_COLOR[d.state],
      title: dayTitle(d.day, this.i18n.lang()),
      logged: d.logged,
      expected: d.expected,
      note: dayNote(d, (k, p) => this.i18n.t(k, p)),
      more: this.store.attention().length - 1,
    };
  });

  /** "Oct 29, Oct 30" (at most four, then "+2"). */
  protected days(days: readonly string[]): string {
    const fmt = new Intl.DateTimeFormat(this.i18n.lang(), { month: 'short', day: 'numeric' });
    const shown = days.slice(0, 4).map((d) => fmt.format(parseDay(d)));
    return days.length > 4 ? `${shown.join(', ')} +${days.length - 4}` : shown.join(', ');
  }

  /** Under each to-do: which days, or why. */
  protected detail(t: TodoDto): string {
    const s = this.summary();
    if (!s) return '';
    switch (t.kind) {
      case 'fillDays':
        return this.days(s.behindDays);
      case 'addTickets':
        return this.days(s.missingDays);
      case 'reviewDrafts':
        return this.days(s.draftDays);
      case 'logInTempo':
        return this.i18n.t('overview.todo.logInTempoDetail');
    }
  }

  protected open(t: TodoDto) {
    return t.day ? this.router.navigate(['/days', t.day]) : this.router.navigate(['/by-ticket']);
  }
}
