import { ChangeDetectionStrategy, Component, inject, model } from '@angular/core';
import { API, type ExportFormat } from '../api/api';
import { I18n } from '../i18n/t.pipe';
import { ShellStore } from '../state/shell.store';
import { ExportSheetComponent } from '../ui/export-sheet/export-sheet.component';

/**
 * Export, the flow around the export sheet: copy or save what it chose, or mark it exported. Sending to Tempo
 * is not here: it goes one day at a time, by hand, from the Days page. Outcomes go to the status bar.
 */
@Component({
  selector: 'wl-export-flow',
  imports: [ExportSheetComponent],
  templateUrl: './export-flow.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ExportFlowComponent {
  readonly open = model(false);

  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);

  protected async copyMarkdown(ids: number[]) {
    const copied = await this.store.attempt(async () => {
      await navigator.clipboard.writeText(await this.api.exportText(ids));
      return true;
    });
    if (copied) this.store.status.set(this.i18n.t('export.copied', { n: ids.length }));
  }

  protected async saveFile(ids: number[], format: ExportFormat) {
    const path = await this.store.attempt(() => this.api.exportFile(ids, format));
    if (path) this.store.status.set(this.i18n.t('export.saved', { path }));
  }
}
