import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { SnChecklistImports } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { TPipe } from '../../i18n/t.pipe';
import type { SetupStep } from '../../shared/setup-steps';

/** The setup still to do, one numbered step per row, each with the button that does it. */
@Component({
  selector: 'wl-setup-checklist',
  imports: [HlmButtonImports, SnChecklistImports, TPipe],
  templateUrl: './setup-checklist.component.html',
  styleUrl: './setup-checklist.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SetupChecklistComponent {
  readonly steps = input.required<SetupStep[]>();
  /** A step's button: the host takes the user there. */
  readonly act = output<SetupStep>();

  protected readonly doneCount = computed(() => this.steps().filter((s) => s.done).length);
  /** The first step not done yet: its button is the accented one. */
  protected readonly next = computed(() => this.steps().find((s) => !s.done) ?? null);
}
