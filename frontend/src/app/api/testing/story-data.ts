import type { MonthDto } from '../api';
import { october, sampleMonth } from './fixtures';

/** Ready-made story inputs derived from the sample October. */
export const oct = october();
export const octDays = new Map(oct.days.map((d) => [d.day, d]));
export const logsOn = (day: string) => oct.worklogs.filter((w) => w.day === day);
export const emptyOctober: MonthDto = { ...sampleMonth('2026-10'), worklogs: [] };
