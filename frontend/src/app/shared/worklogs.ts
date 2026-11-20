/**
 * Display helpers: durations, local dates and times, drag payloads. Pure: no Angular, no Api. What a value means
 * (totals, states, rules) comes from the backend; this only formats it.
 */
import type { WorklogDto } from '../api/api';

// ── Durations ────────────────────────────────────────────────────────────────

/** 5400 → "1h 30m", 1800 → "30m", 3600 → "1h". Rounds to the minute, half away from zero (as Go does). */
export function formatDuration(seconds: number): string {
  const minutes = Math.sign(seconds) * Math.round(Math.abs(seconds) / 60);
  const h = Math.trunc(minutes / 60);
  const m = minutes % 60;
  if (h === 0) return `${m}m`;
  return m === 0 ? `${h}h` : `${h}h ${m}m`;
}

/** 5400 → "1.50 h" */
export function formatHours(seconds: number): string {
  return `${(seconds / 3600).toFixed(2)} h`;
}

// ── Days and times ───────────────────────────────────────────────────────────

/** "HH:MM" in local time from an RFC 3339 timestamp. */
export function hm(iso: string): string {
  const d = new Date(iso);
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

/** RFC 3339 for a local "YYYY-MM-DD" + "HH:MM". */
export function at(day: string, time: string): string {
  const [y, mo, d] = day.split('-').map(Number);
  const [h, mi] = time.split(':').map(Number);
  return new Date(y, mo - 1, d, h, mi).toISOString();
}

export function parseDay(day: string): Date {
  const [y, m, d] = day.split('-').map(Number);
  return new Date(y, m - 1, d);
}

export function dayKey(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/** "Tue, Oct 27, 2026" / "2026. okt. 27., k." */
export function dayTitle(day: string, lang: string): string {
  return new Intl.DateTimeFormat(lang, { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric' }).format(
    parseDay(day),
  );
}

// ── Drag and drop ────────────────────────────────────────────────────────────

/** Dragged entries travel as "worklogs:1,2,3" so unrelated drops are ignored. Entries that can't move stay out. */
export function dragPayload(logs: readonly WorklogDto[]): string {
  return (
    'worklogs:' +
    logs
      .filter((w) => w.canMove)
      .map((w) => w.id)
      .join(',')
  );
}

export function dragIds(text: string): number[] {
  if (!text.startsWith('worklogs:')) return [];
  return text
    .slice('worklogs:'.length)
    .split(',')
    .filter((s) => /^\d+$/.test(s))
    .map(Number);
}
