import type { Source, Status, WorklogDto } from '../api/api';
import { at, dragIds, dragPayload, formatDuration, formatHours } from './worklogs';

let id = 0;
function log(
  day: string,
  time: string,
  seconds: number,
  issueKey: string | null,
  extra: Partial<WorklogDto> = {},
): WorklogDto {
  return {
    id: ++id,
    day,
    start: at(day, time),
    duration: seconds,
    issueKey,
    description: '',
    comment: '',
    source: 'git' as Source,
    estimated: seconds,
    status: 'draft' as Status,
    locked: false,
    tempoId: null,
    // Derived by the backend; these specs don't read them except canMove.
    end: '',
    needsIssue: false,
    needsComment: false,
    tempoText: '',
    focus: null,
    planIssue: null,
    inTempo: false,
    effectiveStatus: 'draft',
    canDelete: true,
    canMove: extra.status !== 'exported',
    startMinute: 0,
    ...extra,
  };
}

describe('durations', () => {
  it.each([
    [0, '0m'],
    [89, '1m'],
    [1800, '30m'],
    [3600, '1h'],
    [5400, '1h 30m'],
    [8 * 3600, '8h'],
  ])('formats %i as %s', (s, text) => expect(formatDuration(s)).toBe(text));

  it('formats decimal hours', () => expect(formatHours(5400)).toBe('1.50 h'));
});

describe('drag payload', () => {
  it('skips exported entries and round-trips', () => {
    const a = { ...log('2026-03-03', '09:00', 60, 'A-1'), id: 1 };
    const b = { ...log('2026-03-03', '10:00', 60, 'A-1', { status: 'exported' }), id: 2 };
    const c = { ...log('2026-03-03', '11:00', 60, 'A-1'), id: 3 };
    expect(dragPayload([a, b, c])).toBe('worklogs:1,3');
    expect(dragIds('worklogs:1,3')).toEqual([1, 3]);
  });

  it.each(['1,2,3', '', 'worklog:1', 'worklogs:'])('ignores %j', (t) => expect(dragIds(t)).toEqual([]));
});
