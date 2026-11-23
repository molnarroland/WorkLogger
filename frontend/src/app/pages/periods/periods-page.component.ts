import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  phosphorArrowRight,
  phosphorFlag,
  phosphorPlus,
  phosphorArrowsClockwise,
  phosphorTrash,
} from '@ng-icons/phosphor-icons/regular';
import { SnCommandBarImports, SnPageCommandsTemplate, SnStatusPage } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { HlmTableImports } from '@spartan-ng/helm/table';
import type { ProjectPeriod } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import { periodColor } from '../../shared/colors';
import { ShellStore } from '../../state/shell.store';
import { PeriodMonthComponent } from '../../ui/period-month/period-month.component';

/** Project periods: a stretch of days booked to one issue (e.g. a migration that runs for three weeks). */
@Component({
  selector: 'wl-periods-page',
  imports: [
    HlmButtonImports,
    HlmInputImports,
    HlmTableImports,
    NgIcon,
    SnPageCommandsTemplate,
    PeriodMonthComponent,
    SnCommandBarImports,
    SnStatusPage,
    TPipe,
  ],
  providers: [provideIcons({ phosphorArrowRight, phosphorFlag, phosphorPlus, phosphorArrowsClockwise, phosphorTrash })],
  templateUrl: './periods-page.component.html',
  styleUrl: './periods-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class PeriodsPageComponent {
  protected readonly store = inject(ShellStore);
  protected readonly periods = computed(() => this.store.settings()?.periods ?? []);
  protected readonly color = periodColor;

  protected add() {
    return this.store.editSettings((api) => api.addPeriod(this.store.month()));
  }

  /** The backend normalises the issue and keeps the end on or after the start. */
  protected update(i: number, patch: Partial<ProjectPeriod>) {
    const p = this.periods()[i];
    return this.store.editSettings((api) => api.updatePeriod({ ...p, ...patch }));
  }

  protected setFrom(i: number, from: string) {
    if (from) return this.update(i, { from });
    return;
  }

  protected setTo(i: number, to: string) {
    if (to) return this.update(i, { to });
    return;
  }

  protected setIssue(i: number, e: Event) {
    return this.update(i, { issueKey: (e.target as HTMLInputElement).value });
  }

  protected remove(i: number) {
    const id = this.periods()[i].id;
    return this.store.editSettings((api) => api.removePeriod(id));
  }
}
