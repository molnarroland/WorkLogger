import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import type { WorklogDto } from '../../api/api';
import { ShellStore } from '../../state/shell.store';
import { CommitSheetComponent } from '../commit-sheet/commit-sheet.component';
import { IssuePickerComponent } from '../issue-picker/issue-picker.component';

/**
 * The sheets an entry's inspector opens: the ticket picker (the chosen ticket is saved on the entry) and a git
 * entry's commits. A page places one and calls it from the inspector's outputs:
 * `<wl-entry-sheets #sheets />` … `(chooseIssue)="sheets.chooseIssue($event)"`.
 */
@Component({
  selector: 'wl-entry-sheets',
  imports: [CommitSheetComponent, IssuePickerComponent],
  template: `
    @if (picking(); as w) {
      <wl-issue-picker
        [(open)]="pickerOpen"
        [day]="w.day"
        [subject]="w.description"
        (picked)="store.updateWorklog({ ...w, issueKey: $event })"
      />
    }
    @if (commitsOf(); as w) {
      <wl-commit-sheet [(open)]="commitsOpen" [worklogId]="w.id" [title]="w.issueKey ?? w.description" />
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EntrySheetsComponent {
  protected readonly store = inject(ShellStore);
  protected readonly picking = signal<WorklogDto | null>(null);
  protected readonly pickerOpen = signal(false);
  protected readonly commitsOf = signal<WorklogDto | null>(null);
  protected readonly commitsOpen = signal(false);

  chooseIssue(w: WorklogDto) {
    this.picking.set(w);
    this.pickerOpen.set(true);
  }

  showCommits(w: WorklogDto) {
    this.commitsOf.set(w);
    this.commitsOpen.set(true);
  }
}
