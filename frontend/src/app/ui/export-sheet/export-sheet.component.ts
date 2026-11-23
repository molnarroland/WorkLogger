import { ChangeDetectionStrategy, Component, computed, input, model, output, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  phosphorCopy,
  phosphorFileText,
  phosphorPaperPlaneTilt,
  phosphorWarning,
} from '@ng-icons/phosphor-icons/regular';
import { SnModalSheet, SnSettingsImports } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmCheckboxImports } from '@spartan-ng/helm/checkbox';
import { HlmLabelImports } from '@spartan-ng/helm/label';
import type { ExportFormat, ExportChoiceDto } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';

/** What to export (filters, counts, the entries skipped for lack of a ticket) and where to. */
@Component({
  selector: 'wl-export-sheet',
  imports: [
    DurationPipe,
    HlmButtonImports,
    HlmCheckboxImports,
    HlmLabelImports,
    NgIcon,
    SnModalSheet,
    SnSettingsImports,
    TPipe,
  ],
  providers: [provideIcons({ phosphorCopy, phosphorFileText, phosphorPaperPlaneTilt, phosphorWarning })],
  templateUrl: './export-sheet.component.html',
  styleUrl: './export-sheet.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ExportSheetComponent {
  readonly open = model(false);
  /** What each setting of the switches sends (MonthDto.exports). */
  readonly choices = input<ExportChoiceDto[]>([]);
  readonly from = input('');
  readonly to = input('');
  readonly copyMarkdown = output<number[]>();
  readonly save = output<{ ids: number[]; format: ExportFormat }>();
  readonly markExported = output<number[]>();
  /** Send the range's reviewed entries to Tempo (the host asks first). */
  readonly sendToTempo = output<void>();

  protected readonly files: ExportFormat[] = ['csv', 'json'];
  protected readonly onlyReviewed = signal(false);
  protected readonly includeExported = signal(false);

  /** The backend's choice for the switches as they are. */
  protected readonly choice = computed(
    () =>
      this.choices().find(
        (c) => c.onlyReviewed === this.onlyReviewed() && c.includeExported === this.includeExported(),
      ) ?? null,
  );
  protected readonly ids = computed(() => this.choice()?.ids ?? []);
  protected readonly seconds = computed(() => this.choice()?.seconds ?? 0);
  protected readonly missing = computed(() => this.choice()?.missing ?? 0);
}
