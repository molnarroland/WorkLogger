import { Injectable, InjectionToken, Pipe, PipeTransform, inject, signal } from '@angular/core';
import type { BackendMessage } from '../api/api';
import en from './en.json';
import hu from './hu.json';

export type Lang = 'en' | 'hu';
/** A plain string, or plural forms picked with Intl.PluralRules from `params.n`. */
type Entry = string | Partial<Record<Intl.LDMLPluralRule, string>>;
type Dict = Record<string, Entry>;

const dicts: Record<Lang, Dict> = { en, hu };

export const I18N_LANG = new InjectionToken<Lang>('I18N_LANG', { factory: () => 'en' });

/** Settings → General → Language: "system" follows the OS. */
export type LanguageChoice = 'system' | Lang;
export const LANGUAGES: LanguageChoice[] = ['system', 'en', 'hu'];

/** The UI language for a choice: "system" (or anything unknown) is the OS language when we have it, else English. */
export function langFor(choice: unknown, osLanguage: string = navigator.language): Lang {
  if (choice === 'en' || choice === 'hu') return choice;
  return osLanguage.toLowerCase().startsWith('hu') ? 'hu' : 'en';
}

const KEY = 'wl.lang';

/** The last chosen language, so the lock screen shows in it (the settings load only after unlocking). */
export function rememberedLang(): Lang {
  try {
    return langFor(localStorage.getItem(KEY));
  } catch {
    return langFor('system');
  }
}

export function rememberLang(choice: LanguageChoice) {
  try {
    localStorage.setItem(KEY, choice);
  } catch {
    /* private window: still applied, just not remembered */
  }
}

@Injectable({ providedIn: 'root' })
export class I18n {
  readonly lang = signal<Lang>(inject(I18N_LANG));

  /** Falls back to English, then to the key itself, so a missing string is visible rather than blank. */
  t(key: string, params: Record<string, string | number> = {}): string {
    const lang = this.lang();
    let entry: Entry | undefined = dicts[lang][key] ?? dicts.en[key];
    if (entry === undefined) return key;
    if (typeof entry !== 'string') {
      const form = new Intl.PluralRules(lang).select(Number(params['n'] ?? 0));
      entry = entry[form] ?? entry.other ?? key;
    }
    return entry.replace(/\{(\w+)\}/g, (m, name: string) => (name in params ? String(params[name]) : m));
  }

  /** A backend message in the UI language; its English text when it has no code. */
  say(m: BackendMessage): string {
    let text = m.code ? this.t('msg.' + m.code, m.params ?? {}) : m.text;
    if (m.skipped) text += ' ' + this.t('msg.skipped', { skipped: m.skipped });
    return text;
  }

  /** What went wrong, for the user: a backend user error (its `cause`) translated, else the error's text. */
  error(e: unknown): string {
    const cause = (e as { cause?: unknown } | null)?.cause;
    if (isMessage(cause)) return this.say(cause);
    return e instanceof Error ? e.message : String(e);
  }
}

/** `{{ 'pane.timesheet' | t }}`, `{{ 'status.needsTicket' | t: { n: 3 } }}`. Impure so a language switch re-renders. */
@Pipe({ name: 't', pure: false })
export class TPipe implements PipeTransform {
  private readonly i18n = inject(I18n);
  transform(key: string, params?: Record<string, string | number>): string {
    return this.i18n.t(key, params);
  }
}

function isMessage(v: unknown): v is BackendMessage {
  return typeof v === 'object' && v !== null && typeof (v as BackendMessage).text === 'string' && 'code' in v;
}
