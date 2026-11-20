import { monthTitle } from './month';

describe('month', () => {
  it('titles in the UI language', () => {
    expect(monthTitle('2026-10', 'en')).toBe('October 2026');
    expect(monthTitle('2026-10', 'hu')).toBe('2026. október');
  });
});
