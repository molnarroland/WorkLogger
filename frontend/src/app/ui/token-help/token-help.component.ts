import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorWarning } from '@ng-icons/phosphor-icons/regular';
import { HlmAlertImports } from '@spartan-ng/helm/alert';
import { TPipe } from '../../i18n/t.pipe';

/** What an API token must have, as a warning box with numbered steps: easy to miss, costly to get wrong. */
@Component({
  selector: 'wl-token-help',
  imports: [HlmAlertImports, NgIcon, TPipe],
  providers: [provideIcons({ phosphorWarning })],
  templateUrl: './token-help.component.html',
  styleUrl: './token-help.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TokenHelpComponent {
  /** i18n keys. */
  readonly title = input.required<string>();
  readonly steps = input.required<readonly string[]>();
}

export const JIRA_TOKEN_STEPS = [
  'jira.tokenStep.create',
  'jira.tokenStep.email',
  'jira.tokenStep.classic',
  'jira.tokenStep.scoped',
];
export const TEMPO_TOKEN_STEPS = [
  'tempo.tokenStep.create',
  'tempo.tokenStep.view',
  'tempo.tokenStep.manage',
  'tempo.tokenStep.url',
];
export const CONFLUENCE_TOKEN_STEPS = [
  'confluence.tokenStep.linked',
  'confluence.tokenStep.classic',
  'confluence.tokenStep.scoped',
  'confluence.tokenStep.own',
];
export const EMAIL_PASSWORD_STEPS = [
  'email.passwordStep.read',
  'email.passwordStep.app',
  'email.passwordStep.microsoft',
  'email.passwordStep.server',
];
