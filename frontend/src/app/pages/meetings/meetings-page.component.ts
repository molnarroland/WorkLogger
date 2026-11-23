import { ChangeDetectionStrategy, Component, computed, inject, input, resource, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  phosphorCalendarDots,
  phosphorLightbulb,
  phosphorArrowsClockwise,
  phosphorUsers,
} from '@ng-icons/phosphor-icons/regular';
import { Router } from '@angular/router';
import { SnCommandBarImports, SnPageCommandsTemplate, SnStatusPage } from '@spartan-native/desktop-ui';
import { SnDialogActionsImports } from '@spartan-native/helm';
import { HlmAlertImports } from '@spartan-ng/helm/alert';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmCheckboxImports } from '@spartan-ng/helm/checkbox';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { HlmTableImports } from '@spartan-ng/helm/table';
import { API, type MeetingDto, type SettingsDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { dayTitle, hm } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { clearFocus } from '../../shared/focus';
import { ScrollIntoViewDirective } from '../../ui/scroll-into-view.directive';
import { DayFilterComponent } from '../../ui/day-filter/day-filter.component';

/** The month's meetings: tick what to log; a ticket for a meeting without one becomes a rule. Setup: Settings → Calendars. */
@Component({
  selector: 'wl-meetings-page',
  imports: [
    SnPageCommandsTemplate,
    DayFilterComponent,
    ScrollIntoViewDirective,
    DurationPipe,
    HlmAlertImports,
    HlmButtonImports,
    HlmCheckboxImports,
    HlmInputImports,
    HlmTableImports,
    NgIcon,
    SnCommandBarImports,
    SnDialogActionsImports,
    SnStatusPage,
    TPipe,
  ],
  providers: [provideIcons({ phosphorCalendarDots, phosphorLightbulb, phosphorArrowsClockwise, phosphorUsers })],
  templateUrl: './meetings-page.component.html',
  styleUrl: './meetings-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MeetingsPageComponent {
  protected readonly store = inject(ShellStore);
  protected readonly api = inject(API);
  private readonly router = inject(Router);
  private readonly i18n = inject(I18n);
  private readonly version = signal(0);
  protected readonly suggestedKey = signal('');

  protected readonly meetings = resource({
    params: () => (this.store.range().from ? { ...this.store.range(), v: this.version() } : undefined),
    loader: ({ params }) => this.api.meetings(params.from, params.to),
  });

  /** The title that comes back most often without a ticket (at least twice). */
  /** The untagged title worth a rule (the backend's pick). */
  protected readonly suggestion = computed(() => this.meetings.value()?.suggestion ?? null);
  /** `?day=`: only that day's meetings (a link from a day's panel). */
  readonly day = input<string>();
  /** `?focus=`: the meeting to point at (its calendar event id). */
  readonly focus = input<string>();
  protected readonly list = computed(() => {
    const all = this.meetings.value()?.meetings ?? [];
    const day = this.day();
    return day ? all.filter((m) => m.day === day) : all;
  });

  protected showAll() {
    return clearFocus(this.router);
  }

  protected when(m: MeetingDto) {
    return `${dayTitle(m.day, this.i18n.lang())} · ${hm(m.start)}–${hm(m.end)}`;
  }

  protected async saveAndRefresh(patch: Partial<SettingsDto>) {
    await this.store.saveSettings(patch);
    this.version.update((v) => v + 1);
  }

  protected async addRule(title: string, issueKey: string) {
    await this.store.editSettings((api) => api.addMeetingRule(title, issueKey));
    this.version.update((v) => v + 1);
  }

  protected openSetup() {
    this.router.navigate([], { queryParams: { tab: 'setup' }, queryParamsHandling: 'merge' });
  }

  /** A ticket for a title becomes a rule; then the drafts are rebuilt so the month picks it up. */
  protected async makeRule(title: string, text: string) {
    const key = text.trim(); // the backend normalises it
    if (!key) return;
    this.suggestedKey.set('');
    await this.addRule(title, key);
    await this.store.run('rebuild');
  }

  protected async skip(id: string, skipped: boolean) {
    await this.store.attempt(() => this.api.setMeetingSkipped(id, skipped));
    this.meetings.reload();
  }
}
