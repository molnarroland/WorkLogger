import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorCheckCircle, phosphorMinusCircle, phosphorXCircle } from '@ng-icons/phosphor-icons/regular';
import { SnSettingsImports } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { HlmLabelImports } from '@spartan-ng/helm/label';
import { HlmSelectImports } from '@spartan-ng/helm/select';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import type { CheckLine, JiraAccount, TokenKind, WorklogProvider } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { JIRA_TOKEN_STEPS, TokenHelpComponent } from '../token-help/token-help.component';

/** One Jira profile: sign-in, where its worklogs live, its projects, and the connection check. */
@Component({
  selector: 'wl-jira-account-form',
  imports: [
    HlmButtonImports,
    HlmInputImports,
    HlmLabelImports,
    HlmSelectImports,
    HlmSpinnerImports,
    NgIcon,
    SnSettingsImports,
    TPipe,
    TokenHelpComponent,
  ],
  providers: [provideIcons({ phosphorCheckCircle, phosphorMinusCircle, phosphorXCircle })],
  templateUrl: './jira-account-form.component.html',
  styleUrl: './jira-account-form.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class JiraAccountFormComponent {
  private readonly i18n = inject(I18n);
  readonly account = input.required<JiraAccount>();
  /** A Jira token is saved (its value never comes back): the field offers to replace it. */
  readonly hasJiraToken = input(false);
  /** The Tempo profile that links this account, if any (its worklogs then go to Tempo). */
  readonly tempo = input<string | null>(null);
  readonly checkLines = input<CheckLine[]>([]);
  readonly checking = input(false);
  readonly loadingProjects = input(false);
  readonly changed = output<JiraAccount>();
  readonly token = output<{ kind: TokenKind; value: string }>();
  readonly readProjects = output<void>();
  readonly check = output<TokenKind>();

  /** Tempo is a profile of its own (it links the account), so here only Jira's own worklogs or another tool. */
  protected readonly providers: WorklogProvider[] = ['jira', 'other'];
  protected readonly providerLabel = (p: unknown) => this.i18n.t('jira.provider.' + String(p));
  protected readonly tokenSteps = JIRA_TOKEN_STEPS;
  protected readonly id = computed(() => `ja-${this.account().id}`);

  protected val(e: Event) {
    return (e.target as HTMLInputElement).value.trim();
  }

  protected setProvider(p: unknown) {
    if (p === 'jira' || p === 'other') this.emit({ provider: p });
  }

  protected emit(patch: Partial<JiraAccount>) {
    this.changed.emit({ ...this.account(), ...patch });
  }
}
