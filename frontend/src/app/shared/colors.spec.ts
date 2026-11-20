import { NO_TICKET_COLOR, PERIOD_COLORS, SOURCE_COLOR, periodColor, toBarParts } from './colors';

describe('colors', () => {
  it('draws the backend parts in their source colours, no ticket in orange', () => {
    expect(
      toBarParts([
        { kind: 'git', seconds: 7200 },
        { kind: 'noTicket', seconds: 1800 },
      ]),
    ).toEqual([
      { color: SOURCE_COLOR.git, seconds: 7200, label: 'source.git' },
      { color: NO_TICKET_COLOR, seconds: 1800, label: 'source.noTicket' },
    ]);
  });

  it('cycles the period palette', () => expect(periodColor(PERIOD_COLORS.length)).toBe(periodColor(0)));
});
