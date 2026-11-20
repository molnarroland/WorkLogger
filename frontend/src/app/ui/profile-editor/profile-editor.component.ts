import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideCircleCheck, lucideCircleMinus, lucideCircleX } from '@ng-icons/lucide';
import { SnSettingsImports } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { HlmLabelImports } from '@spartan-ng/helm/label';
import { HlmSelectImports } from '@spartan-ng/helm/select';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import type { CheckLine, Profile } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import {
  CONFLUENCE_TOKEN_STEPS,
  EMAIL_PASSWORD_STEPS,
  TEMPO_TOKEN_STEPS,
  TokenHelpComponent,
} from '../token-help/token-help.component';
import { SignInBadgeComponent } from '../sign-in-badge/sign-in-badge.component';

export interface JiraChoice {
  id: string;
  title: string;
}

/**
 * One Google, HRmaster, Tempo or Confluence profile: its fields, and its sign-in (Google, HRmaster) or token and
 * check (Tempo, Confluence). Every field saves when it loses focus.
 */
@Component({
  selector: 'wl-profile-editor',
  imports: [
    HlmButtonImports,
    HlmInputImports,
    HlmLabelImports,
    HlmSelectImports,
    HlmSpinnerImports,
    NgIcon,
    SignInBadgeComponent,
    SnSettingsImports,
    TPipe,
    TokenHelpComponent,
  ],
  providers: [provideIcons({ lucideCircleCheck, lucideCircleMinus, lucideCircleX })],
  templateUrl: './profile-editor.component.html',
  styleUrl: './profile-editor.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfileEditorComponent {
  private readonly i18n = inject(I18n);
  readonly profile = input.required<Profile>();
  /** Google, HRmaster: the window's last sign-in check. */
  readonly signedIn = input<boolean | null>(null);
  /**
   * Tempo: the Jira profiles it can log for (those without another Tempo profile, and its own).
   * Confluence: the Jira profiles it can use the site, email and token of.
   */
  readonly jiraAccounts = input<JiraChoice[]>([]);
  /** Its Tempo token, own Confluence token or mail password is saved (its value never comes back). */
  readonly hasToken = input(false);
  readonly checkLines = input<CheckLine[]>([]);
  readonly checking = input(false);
  readonly changed = output<Profile>();
  readonly signIn = output<void>();
  readonly signOut = output<void>();
  readonly token = output<string>();
  readonly check = output<void>();

  protected readonly id = computed(() => `pf-${this.profile().id}`);
  protected readonly tokenSteps = TEMPO_TOKEN_STEPS;
  protected readonly confluenceSteps = CONFLUENCE_TOKEN_STEPS;
  protected readonly emailSteps = EMAIL_PASSWORD_STEPS;
  /** The "no Jira profile" choice of a Confluence profile (a select item can't be ""). */
  protected readonly noJira = 'none';
  protected readonly jiraTitle = (id: unknown) =>
    id === this.noJira || id === ''
      ? this.i18n.t('confluence.noJira')
      : (this.jiraAccounts().find((a) => a.id === id)?.title ?? '');
  /** Confluence: the linked Jira profile gives the site and email, so those fields only override it. */
  protected readonly linked = computed(() => !!this.profile().jiraAccountId);
  protected readonly ready = computed(() => {
    const p = this.profile();
    return p.kind === 'hrmaster' ? /^https?:\/\/\S+/.test(p.url) : true;
  });

  protected val(e: Event) {
    return (e.target as HTMLInputElement).value.trim();
  }

  protected setJira(id: unknown) {
    if (typeof id !== 'string') return;
    this.emit({ jiraAccountId: id === this.noJira ? '' : id });
  }

  protected emit(patch: Partial<Profile>) {
    const p = this.profile();
    if (Object.entries(patch).every(([k, v]) => p[k as keyof Profile] === v)) return;
    this.changed.emit({ ...p, ...patch });
  }

  protected kindTitle() {
    return this.i18n.t('profiles.kind.' + this.profile().kind);
  }
}
