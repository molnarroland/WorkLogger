import { ChangeDetectionStrategy, Component, computed, input, output, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorArrowRight, phosphorTrash } from '@ng-icons/phosphor-icons/regular';
import { SnDialogActionsImports } from '@spartan-native/helm';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { TPipe } from '../../i18n/t.pipe';

export interface RuleItem {
  /** What matches: a title fragment, a day label, a project key, a conversation. */
  label: string;
  issueKey: string | null;
}

/**
 * "“Sprint planning” → TEAM-7" lines with remove buttons, and a pair of fields to add one (Enter adds). Used for
 * meeting rules, absence rules, chat rules and Jira projects' catch-all issues.
 */
@Component({
  selector: 'wl-rule-list',
  imports: [HlmButtonImports, HlmInputImports, NgIcon, SnDialogActionsImports, TPipe],
  providers: [provideIcons({ phosphorArrowRight, phosphorTrash })],
  templateUrl: './rule-list.component.html',
  styleUrl: './rule-list.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RuleListComponent {
  readonly items = input.required<RuleItem[]>();
  /** Text of the add button; empty hides the add row. */
  readonly addLabel = input('');
  readonly labelPlaceholder = input('');
  readonly issuePlaceholder = input('');
  /** The issue may stay empty (a project without a catch-all issue). */
  readonly issueOptional = input(false);
  /** Project keys: mono and upper case, no quotes. */
  readonly mono = input(false);
  readonly quoted = input(true);
  readonly add = output<RuleItem>();
  readonly remove = output<number>();

  protected readonly newLabel = signal('');
  protected readonly newIssue = signal('');
  protected readonly valid = computed(
    () => !!this.newLabel().trim() && (this.issueOptional() || !!this.newIssue().trim()),
  );

  protected submit() {
    if (!this.valid()) return;
    const label = this.newLabel().trim();
    // The backend normalises keys; upper case here only so the list shows what it will save.
    this.add.emit({ label: this.mono() ? label.toUpperCase() : label, issueKey: this.newIssue().trim() || null });
    this.newLabel.set('');
    this.newIssue.set('');
  }
}
