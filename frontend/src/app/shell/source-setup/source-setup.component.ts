import { ChangeDetectionStrategy, Component, computed, inject, input } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorCheckCircle, phosphorCircleDashed } from '@ng-icons/phosphor-icons/regular';
import type { ProfileKind } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import { RepoSetupComponent } from '../../pages/repositories/repo-setup.component';
import { CalendarSettingsComponent } from '../../pages/settings/calendar-settings.component';
import { ProfilesSettingsComponent } from '../../pages/settings/profiles-settings.component';
import { SETUP_PANE, toSetupSteps } from '../../shared/setup-steps';
import { ShellStore } from '../../state/shell.store';

/** The profile each source reads with; Repositories and Meetings have forms of their own. */
const PROFILE: Record<string, ProfileKind> = {
  sessions: 'hrmaster',
  chats: 'googleChat',
  jira: 'jira',
  confluence: 'confluence',
  email: 'email',
};

/** A source page's Setup tab: what the source still needs, then the form where it is set up. */
@Component({
  selector: 'wl-source-setup',
  imports: [CalendarSettingsComponent, NgIcon, ProfilesSettingsComponent, RepoSetupComponent, TPipe],
  providers: [provideIcons({ phosphorCheckCircle, phosphorCircleDashed })],
  templateUrl: './source-setup.component.html',
  styleUrl: './source-setup.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SourceSetupComponent {
  private readonly store = inject(ShellStore);
  /** The source page's id (a pane id). */
  readonly source = input.required<string>();
  protected readonly profile = computed(() => PROFILE[this.source()] ?? null);
  /** This source's setup steps, done or not. */
  protected readonly steps = computed(() =>
    toSetupSteps(this.store.setup()).filter((s) => SETUP_PANE[s.id] === this.source()),
  );
}
