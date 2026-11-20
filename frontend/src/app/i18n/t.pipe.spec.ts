import { TestBed } from '@angular/core/testing';
import en from './en.json';
import hu from './hu.json';
import { I18N_LANG, I18n, langFor } from './t.pipe';

function i18n(lang: 'en' | 'hu') {
  TestBed.configureTestingModule({ providers: [{ provide: I18N_LANG, useValue: lang }] });
  return TestBed.inject(I18n);
}

describe('I18n', () => {
  it('picks plural forms and interpolates', () => {
    const t = i18n('en');
    expect(t.t('status.needsTicket', { n: 1 })).toBe('1 needs a ticket');
    expect(t.t('status.needsTicket', { n: 3 })).toBe('3 need a ticket');
    expect(t.t('status.inTempo', { entered: 38, total: 69 })).toBe('38 of 69 ticket-days in Tempo');
  });

  it('switches language at runtime', () => {
    const t = i18n('en');
    t.lang.set('hu');
    expect(t.t('pane.review')).toBe('Átnézés');
  });

  it('falls back to the key when a string is missing', () => {
    expect(i18n('hu').t('no.such.key')).toBe('no.such.key');
  });

  it('has the same keys in every language', () => {
    expect(Object.keys(hu).sort()).toEqual(Object.keys(en).sort());
  });

  it('takes the chosen language, or the OS language when it is one we have', () => {
    expect(langFor('hu', 'en-US')).toBe('hu');
    expect(langFor('en', 'hu-HU')).toBe('en');
    expect(langFor('system', 'hu-HU')).toBe('hu');
    expect(langFor('system', 'de-DE')).toBe('en');
    expect(langFor('klingon', 'hu')).toBe('hu');
  });
});
