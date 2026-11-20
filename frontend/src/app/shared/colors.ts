/**
 * Colour tokens for what the backend classifies: sources, day states, day progress, periods. Presentation only:
 * the states and totals come from Go (internal/view); this maps them to `--wl-*` / `--sn-color-*` tokens.
 */
import type { DayProgress, DayState, PartDto, Source } from '../api/api';

/** A stacked-bar segment ready to draw. */
export interface BarPart {
  /** CSS colour, always a token (`--wl-*` in styles.css, or a `--sn-color-*`). */
  color: string;
  seconds: number;
  /** i18n key of the part's name */
  label: string;
  /** The icon the day cards show for it, if any (a lucide name). */
  icon?: string;
}

export const SOURCE_COLOR: Record<Source, string> = {
  git: 'var(--wl-source-coding)',
  jira: 'var(--wl-source-jira)',
  confluence: 'var(--wl-source-confluence)',
  email: 'var(--wl-source-email)',
  meeting: 'var(--wl-source-meeting)',
  chat: 'var(--wl-source-chat)',
  absence: 'var(--wl-source-leave)',
  period: 'var(--wl-source-period)',
  manual: 'var(--wl-source-manual)',
};

/** Orange always means "needs a ticket". */
export const NO_TICKET_COLOR = 'var(--wl-needs-ticket)';

/** The backend's bar parts (a source, or `noTicket`) as drawable segments. */
export function toBarParts(parts: readonly PartDto[]): BarPart[] {
  return parts.map((p) =>
    p.kind === 'noTicket'
      ? { color: NO_TICKET_COLOR, seconds: p.seconds, label: 'source.noTicket' }
      : { color: SOURCE_COLOR[p.kind], seconds: p.seconds, label: `source.${p.kind}` },
  );
}

/** Colour of a triage state: the day-state tokens. */
export const TRIAGE_COLOR: Record<DayState, string> = {
  empty: 'var(--wl-state-empty)',
  partial: 'var(--wl-state-partial)',
  noTicket: NO_TICKET_COLOR,
  draft: 'var(--sn-color-primary)',
  done: 'var(--wl-state-complete)',
  upcoming: 'var(--wl-state-idle)',
  free: 'var(--wl-state-idle)',
  absence: 'var(--wl-state-leave)',
};

/** Which state colour a day's progress gets (DESIGN.md, day states). */
export const PROGRESS_COLOR: Record<DayProgress, string> = {
  absence: 'var(--wl-state-leave)',
  free: 'var(--wl-state-idle)',
  upcoming: 'var(--wl-state-idle)',
  extra: 'var(--wl-state-extra)',
  complete: 'var(--wl-state-complete)',
  empty: 'var(--wl-state-empty)',
  partial: 'var(--wl-state-partial)',
};

/** Period colours by position (PeriodsView.palette), the same on the Periods page and the Timesheet. */
export const PERIOD_COLORS = [
  'var(--sn-color-primary)',
  'var(--wl-source-meeting)',
  'var(--wl-source-chat)',
  'var(--wl-source-leave)',
  'var(--wl-source-period)',
  'var(--wl-state-complete)',
  'var(--wl-state-partial)',
];

export function periodColor(index: number): string {
  return PERIOD_COLORS[index % PERIOD_COLORS.length];
}
