import { STYLED_SKINS, TEXT_SCALES, type AppearanceState, type StyledSkin } from '@spartan-native/theme';

/**
 * Settings → General: the look on top of what the OS says. Every field has "system", meaning: take it from the OS
 * through the Spartan Native host. Anything else overrides it.
 */
export type Appearance = 'system' | 'light' | 'dark';
export type AccentColor = 'system' | 'blue' | 'green' | 'red' | 'orange' | 'purple';
/** Which OS the window looks like. */
export type SkinChoice = 'system' | StyledSkin;
export type Transparency = 'system' | 'full' | 'reduced';
export type Contrast = 'system' | 'normal' | 'more';
export type Motion = 'system' | 'full' | 'reduced';
/** 0 = the OS text size. */
export type TextScale = 0 | (typeof TEXT_SCALES)[number];

export const APPEARANCES: Appearance[] = ['system', 'light', 'dark'];
export const ACCENTS: AccentColor[] = ['system', 'blue', 'green', 'red', 'orange', 'purple'];
export const SKINS: SkinChoice[] = ['system', ...STYLED_SKINS];
export const TRANSPARENCIES: Transparency[] = ['system', 'full', 'reduced'];
export const CONTRASTS: Contrast[] = ['system', 'normal', 'more'];
export const MOTIONS: Motion[] = ['system', 'full', 'reduced'];
export const TEXT_SCALE_CHOICES: TextScale[] = [0, ...TEXT_SCALES];

/** The fixed accents, as Spartan Native derives a whole palette from one colour. */
export const ACCENT_HEX: Record<Exclude<AccentColor, 'system'>, string> = {
  blue: '#007aff',
  green: '#34c759',
  red: '#ff3b30',
  orange: '#ff9500',
  purple: '#af52de',
};

export interface Look {
  appearance: Appearance;
  accent: AccentColor;
  skin: SkinChoice;
  transparency: Transparency;
  contrast: Contrast;
  motion: Motion;
  textScale: TextScale;
}

export const SYSTEM_LOOK: Look = {
  appearance: 'system',
  accent: 'system',
  skin: 'system',
  transparency: 'system',
  contrast: 'system',
  motion: 'system',
  textScale: 0,
};

/** The look as saved in the settings; anything unknown (an older or newer build) follows the system. */
export function lookOf(s: Partial<Record<keyof Look | 'accentColor', unknown>>): Look {
  const pick = <T>(v: unknown, allowed: readonly T[], fallback: T) => (allowed.includes(v as T) ? (v as T) : fallback);
  return {
    appearance: pick(s.appearance, APPEARANCES, 'system'),
    accent: pick(s.accentColor ?? s.accent, ACCENTS, 'system'),
    skin: pick(s.skin, SKINS, 'system'),
    transparency: pick(s.transparency, TRANSPARENCIES, 'system'),
    contrast: pick(s.contrast, CONTRASTS, 'system'),
    motion: pick(s.motion, MOTIONS, 'system'),
    textScale: pick(s.textScale, TEXT_SCALE_CHOICES, 0),
  };
}

/** What the user picked, as overrides on top of the OS values. Empty = follow the system. */
export function lookOverrides(look: Look): Partial<AppearanceState> {
  const o: Partial<AppearanceState> = {};
  if (look.appearance !== 'system') o.scheme = look.appearance;
  if (look.accent !== 'system' && look.accent in ACCENT_HEX) o.accent = ACCENT_HEX[look.accent];
  if (look.skin !== 'system') o.skin = look.skin;
  if (look.transparency !== 'system') o.reduceTransparency = look.transparency === 'reduced';
  if (look.contrast !== 'system') o.contrast = look.contrast;
  if (look.motion !== 'system') o.reduceMotion = look.motion === 'reduced';
  if (look.textScale) o.textScale = look.textScale;
  return o;
}

const KEY = 'wl.look';

/** The last look, so the window opens in it before the settings load (they only load after unlocking). */
export function rememberedLook(): Partial<AppearanceState> {
  try {
    const saved = JSON.parse(localStorage.getItem(KEY) ?? 'null') as Partial<Look> | null;
    if (saved && typeof saved === 'object') return lookOverrides(lookOf(saved));
  } catch {
    /* nothing remembered, or no storage */
  }
  return {};
}

export function rememberLook(look: Look) {
  try {
    localStorage.setItem(KEY, JSON.stringify(look));
  } catch {
    /* private window: still applied, just not remembered */
  }
}
