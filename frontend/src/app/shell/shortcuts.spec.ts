import { matchShortcut } from './shortcuts';

const key = (code: string, mods: Partial<KeyboardEvent> = {}) =>
  ({ key: '', code, ctrlKey: true, metaKey: false, shiftKey: false, altKey: false, ...mods }) as KeyboardEvent;

describe('matchShortcut', () => {
  it('maps the Mac shortcuts with Ctrl for Cmd', () => {
    expect(matchShortcut(key('Digit1'))).toEqual({ kind: 'page', index: 0 });
    expect(matchShortcut(key('Digit9'))).toEqual({ kind: 'page', index: 8 });
    expect(matchShortcut(key('KeyR'))).toEqual({ kind: 'collect' });
    expect(matchShortcut(key('KeyR', { shiftKey: true }))).toEqual({ kind: 'rebuild' });
    expect(matchShortcut(key('BracketLeft'))).toEqual({ kind: 'prevMonth' });
    expect(matchShortcut(key('BracketRight'))).toEqual({ kind: 'nextMonth' });
    expect(matchShortcut(key('KeyL'))).toEqual({ kind: 'logInTempo' });
  });

  it('ignores anything without plain Ctrl', () => {
    expect(matchShortcut(key('KeyR', { ctrlKey: false }))).toBeNull();
    expect(matchShortcut(key('KeyR', { altKey: true }))).toBeNull();
    expect(matchShortcut(key('Digit0'))).toBeNull();
    expect(matchShortcut(key('KeyC'))).toBeNull();
  });

  it('uses Cmd on macOS', () => {
    expect(matchShortcut(key('KeyR', { ctrlKey: false, metaKey: true }), 'macos')).toEqual({ kind: 'collect' });
    expect(matchShortcut(key('KeyR'), 'macos')).toBeNull();
  });
});
