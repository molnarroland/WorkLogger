import { describe, expect, it } from 'vitest';
import { ACCENT_HEX, SYSTEM_LOOK, lookOf, lookOverrides } from './appearance';

describe('lookOverrides', () => {
  it('follows the system when nothing is picked', () => {
    expect(lookOverrides(SYSTEM_LOOK)).toEqual({});
  });

  it('overrides only what the user picked', () => {
    expect(lookOverrides({ ...SYSTEM_LOOK, appearance: 'dark' })).toEqual({ scheme: 'dark' });
    expect(lookOverrides({ ...SYSTEM_LOOK, accent: 'green' })).toEqual({ accent: ACCENT_HEX.green });
    expect(lookOverrides({ ...SYSTEM_LOOK, skin: 'cros' })).toEqual({ skin: 'cros' });
    expect(lookOverrides({ ...SYSTEM_LOOK, transparency: 'reduced', motion: 'full' })).toEqual({
      reduceTransparency: true,
      reduceMotion: false,
    });
    expect(lookOverrides({ ...SYSTEM_LOOK, contrast: 'more', textScale: 1.5 })).toEqual({
      contrast: 'more',
      textScale: 1.5,
    });
  });
});

describe('lookOf', () => {
  it('reads the saved settings and drops unknown values', () => {
    expect(lookOf({ appearance: 'dark', accentColor: 'red', skin: 'gtk', textScale: 1.25 })).toEqual({
      ...SYSTEM_LOOK,
      appearance: 'dark',
      accent: 'red',
      skin: 'gtk',
      textScale: 1.25,
    });
    expect(lookOf({ skin: 'amiga', transparency: 'some', textScale: 3 })).toEqual(SYSTEM_LOOK);
  });
});
