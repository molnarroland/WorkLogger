import { DOCUMENT, Injectable, inject } from '@angular/core';
import type { PlatformFamily } from '@spartan-native/theme';

export type Shortcut =
  | { kind: 'page'; index: number }
  | { kind: 'collect' }
  | { kind: 'rebuild' }
  | { kind: 'prevMonth' }
  | { kind: 'nextMonth' }
  | { kind: 'logInTempo' };

/** The same map in desktop-ui's portable notation (`mod` = ⌘ on macOS, Ctrl elsewhere), for menus and `snShortcut`. */
export const ACCELERATORS = {
  collect: 'mod+r',
  rebuild: 'mod+shift+r',
  prevMonth: 'mod+[',
  nextMonth: 'mod+]',
  logInTempo: 'mod+l',
  page: (index: number) => `mod+${index + 1}`,
} as const;

type Keys = Pick<KeyboardEvent, 'key' | 'code' | 'ctrlKey' | 'metaKey' | 'shiftKey' | 'altKey'>;

/**
 * ⌘ on macOS, Ctrl elsewhere. Uses `code` for brackets and digits so keyboard layouts (HU)
 * don't matter.
 */
export function matchShortcut(e: Keys, family: PlatformFamily = 'windows'): Shortcut | null {
  const mac = family === 'macos';
  const mod = mac ? e.metaKey && !e.ctrlKey : e.ctrlKey && !e.metaKey;
  if (!mod || e.altKey) return null;
  const digit = /^Digit([1-9])$/.exec(e.code);
  if (digit && !e.shiftKey) return { kind: 'page', index: Number(digit[1]) - 1 };
  switch (e.code) {
    case 'KeyR':
      return { kind: e.shiftKey ? 'rebuild' : 'collect' };
    case 'BracketLeft':
      return e.shiftKey ? null : { kind: 'prevMonth' };
    case 'BracketRight':
      return e.shiftKey ? null : { kind: 'nextMonth' };
    case 'KeyL':
      return e.shiftKey ? null : { kind: 'logInTempo' };
  }
  return null;
}

/** One document-level listener. `listen` returns the unsubscribe. */
@Injectable({ providedIn: 'root' })
export class ShortcutService {
  private readonly doc = inject(DOCUMENT);

  listen(family: () => PlatformFamily, handler: (s: Shortcut) => void): () => void {
    const onKey = (e: KeyboardEvent) => {
      const s = matchShortcut(e, family());
      if (!s) return;
      e.preventDefault(); // Ctrl+R / ⌘R would otherwise reload the webview
      handler(s);
    };
    this.doc.addEventListener('keydown', onKey);
    return () => this.doc.removeEventListener('keydown', onKey);
  }
}
