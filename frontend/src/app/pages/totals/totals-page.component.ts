import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { provideIcons } from '@ng-icons/core';
import { lucideChartBar } from '@ng-icons/lucide';
import { SnPageCommandsTemplate, SnStatusPage } from '@spartan-native/desktop-ui';
import { HlmAccordionImports } from '@spartan-ng/helm/accordion';
import { HlmTableImports } from '@spartan-ng/helm/table';
import { HlmToggleGroupImports } from '@spartan-ng/helm/toggle-group';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { type BarPart, SOURCE_COLOR, toBarParts } from '../../shared/colors';
import { monthTitle } from '../../shared/month';
import type { Period, ReportGroupDto } from '../../api/api';
import { formatHours, parseDay } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { SourceBarComponent } from '../../ui/source-bar/source-bar.component';
import { SourceLegendComponent } from '../../ui/source-legend/source-legend.component';
import { StackedBarComponent } from '../../ui/stacked-bar/stacked-bar.component';

const TICKET_LEGEND: BarPart[] = [
  { color: 'var(--wl-state-complete)', seconds: 0, label: 'totals.inTempo' },
  { color: 'var(--sn-color-primary)', seconds: 0, label: 'totals.stillToEnter' },
  { color: 'var(--wl-needs-ticket)', seconds: 0, label: 'totals.needsTicket' },
];

/** The month's time by ticket (in Tempo vs still to enter), by source, and per day, week or month. */
@Component({
  selector: 'wl-totals-page',
  imports: [
    SnPageCommandsTemplate,
    DurationPipe,
    HlmAccordionImports,
    HlmTableImports,
    HlmToggleGroupImports,
    SnStatusPage,
    SourceBarComponent,
    SourceLegendComponent,
    StackedBarComponent,
    TPipe,
  ],
  providers: [provideIcons({ lucideChartBar })],
  templateUrl: './totals-page.component.html',
  styleUrl: './totals-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TotalsPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly i18n = inject(I18n);
  protected readonly periods: Period[] = ['day', 'week', 'month'];
  protected readonly period = signal<Period>('week');
  protected readonly ticketLegend = TICKET_LEGEND;
  protected readonly hours = formatHours;

  protected readonly total = computed(() => this.store.summary()?.logged ?? 0);
  protected readonly range = this.store.range;
  protected readonly sourceParts = computed(() => toBarParts(this.store.summary()?.barParts ?? []));
  protected readonly ticketTop = computed(() => Math.max(1, ...this.store.tickets().map((g) => g.seconds)));
  protected readonly ticketRows = computed(() =>
    this.store.tickets().map((g) => {
      const rest: BarPart = g.key
        ? { color: 'var(--sn-color-primary)', seconds: g.remainingSeconds, label: 'totals.stillToEnter' }
        : { color: 'var(--wl-needs-ticket)', seconds: g.remainingSeconds, label: 'totals.needsTicket' };
      return {
        key: g.key,
        seconds: g.seconds,
        parts: [{ color: 'var(--wl-state-complete)', seconds: g.enteredSeconds, label: 'totals.inTempo' }, rest],
      };
    }),
  );

  protected readonly groups = computed(() => this.store.data()?.reports[this.period()] ?? []);
  protected readonly labels = computed(() => this.groups().map((g) => this.label(g)));
  protected readonly columns = computed(() =>
    this.groups().map((g, i) => ({
      label: this.labels()[i],
      parts: g.bySource.map((s) => ({
        color: SOURCE_COLOR[s.source],
        seconds: s.seconds,
        label: `source.${s.source}`,
      })),
    })),
  );

  protected setPeriod(value: unknown) {
    if (value === 'day' || value === 'week' || value === 'month') this.period.set(value);
  }

  private label(g: ReportGroupDto): string {
    const lang = this.i18n.lang();
    const d = parseDay(g.start);
    switch (this.period()) {
      case 'day':
        return new Intl.DateTimeFormat(lang, { weekday: 'short', day: 'numeric', month: 'short' }).format(d);
      case 'week': {
        const date = new Intl.DateTimeFormat(lang, { day: 'numeric', month: 'short' }).format(d);
        return this.i18n.t('totals.weekOf', { date });
      }
      case 'month':
        return monthTitle(g.id, lang);
    }
  }
}
