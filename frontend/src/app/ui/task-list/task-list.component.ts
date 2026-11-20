import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideCircleCheck, lucideCircleSlash, lucideCircleX } from '@ng-icons/lucide';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import { TPipe } from '../../i18n/t.pipe';
import type { Task } from '../../shared/task';

/** The status bar's Tasks popup: what this window started, newest first, and how each ended. */
@Component({
  selector: 'wl-task-list',
  imports: [HlmButtonImports, HlmSpinnerImports, NgIcon, TPipe],
  providers: [provideIcons({ lucideCircleCheck, lucideCircleSlash, lucideCircleX })],
  templateUrl: './task-list.component.html',
  styleUrl: './task-list.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TaskListComponent {
  readonly tasks = input<Task[]>([]);
  readonly cleared = output<void>();

  protected readonly newestFirst = computed(() => [...this.tasks()].reverse());
  protected readonly finished = computed(() => this.tasks().some((t) => t.state !== 'running'));

  protected time(t: string) {
    return new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }
}
