import { provideRouter, withComponentInputBinding, withHashLocation } from '@angular/router';
import { type Skin, provideSpartanNative, staticHost } from '@spartan-native/theme';
import { applicationConfig, componentWrapperDecorator, type Preview } from '@storybook/angular-vite';
import { provideSpartanHlm } from '@spartan-ng/helm/utils';
import { API } from '../src/app/api/api';
import { FakeApi } from '../src/app/api/fake-api';
import { I18N_LANG, type Lang } from '../src/app/i18n/t.pipe';
import { routes } from '../src/app/app.routes';

/** The seven Spartan Native skins, as the OS a story pretends to run on. */
const SKINS: { value: Skin; title: string }[] = [
  { value: 'win', title: 'Windows 11' },
  { value: 'w10', title: 'Windows 10' },
  { value: 'mac', title: 'macOS 26' },
  { value: 'macc', title: 'macOS 13–15' },
  { value: 'gtk', title: 'GNOME' },
  { value: 'qt', title: 'KDE Plasma' },
  { value: 'cros', title: 'ChromeOS' },
];

/** The story's language: its own `parameters.lang` (the hu variants), else the toolbar's. */
const storyLang = (ctx: { parameters: Record<string, unknown>; globals: Record<string, unknown> }): Lang =>
  ctx.parameters['lang'] === 'hu' || (ctx.parameters['lang'] === undefined && ctx.globals['lang'] === 'hu') ? 'hu' : 'en';

const preview: Preview = {
  globalTypes: {
    lang: { toolbar: { title: 'Language', items: ['en', 'hu'], icon: 'globe', dynamicTitle: true } },
    skin: { toolbar: { title: 'OS', items: SKINS, icon: 'browser', dynamicTitle: true } },
    scheme: { toolbar: { title: 'Scheme', items: ['light', 'dark'], icon: 'mirror', dynamicTitle: true } },
    accent: {
      toolbar: {
        title: 'Accent',
        icon: 'paintbrush',
        dynamicTitle: true,
        items: [
          { value: '#0078d4', title: 'Windows blue' },
          { value: '#007aff', title: 'macOS blue' },
          { value: '#3584e4', title: 'GNOME blue' },
          { value: '#9141ac', title: 'Purple' },
          { value: '#26a269', title: 'Green' },
        ],
      },
    },
  },
  initialGlobals: { lang: 'en', skin: 'win', scheme: 'light', accent: '#0078d4' },
  decorators: [
    // Inside the app every page and part sits in the window's <main>; the shell (`parameters.ownMain`) has its own.
    (story, ctx) =>
      ctx.parameters['ownMain']
        ? story()
        : componentWrapperDecorator((s) => `<main style="display: contents">${s}</main>`)(story, ctx),
    // Like the app in a browser: a code-controlled host, here driven by the toolbar. A story's own
    // `parameters.api` (FakeApiOptions) seeds the fake; `parameters.lang` pins the language (hu variants).
    (story, ctx) =>
      applicationConfig({
        providers: [
          provideSpartanHlm(),
          provideSpartanNative({
            host: staticHost({ skin: ctx.globals['skin'], scheme: ctx.globals['scheme'], accent: ctx.globals['accent'] }),
          }),
          provideRouter(routes, withComponentInputBinding(), withHashLocation()),
          {
            provide: API,
            useFactory: () => new FakeApi({ stepMs: 300, language: storyLang(ctx), ...ctx.parameters['api'] }),
          },
          { provide: I18N_LANG, useValue: storyLang(ctx) },
        ],
      })(story, ctx),
  ],
  parameters: {
    layout: 'fullscreen',
    a11y: { test: 'error' },
  },
};

export default preview;
