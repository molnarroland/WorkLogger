import { type ApplicationConfig, provideBrowserGlobalErrorListeners } from '@angular/core';
import { provideRouter, withComponentInputBinding, withHashLocation } from '@angular/router';
import { isWails, wailsHost } from '@spartan-native/host-wails';
import { provideSpartanNative, staticHost } from '@spartan-native/theme';
import { provideSpartanHlm } from '@spartan-ng/helm/utils';
import { API, type Api } from './api/api';
import { WailsApi } from './api/wails-api';
import { I18N_LANG, rememberedLang } from './i18n/t.pipe';
import { rememberedLook } from './shared/appearance';
import { routes } from './app.routes';

/**
 * The only place that knows the runtime. Inside the Wails window the Go backend answers (WailsApi) and the
 * Spartan Native Go service reads the real OS look (wailsHost). In a plain browser (`ng serve`, e2e) the in-memory
 * FakeApi answers and the look is a fixed, code-controlled host. The fake and its sample data are loaded only
 * there, so the app's bundle doesn't carry them.
 */
const inWails = isWails();

export async function appConfig(): Promise<ApplicationConfig> {
  const api: Api = inWails ? new WailsApi() : new (await import('./api/fake-api')).FakeApi();
  return { providers: providers(api) };
}

const providers = (api: Api): ApplicationConfig['providers'] => [
  provideBrowserGlobalErrorListeners(),
  // Hash URLs: the Wails asset server only serves index.html at the root.
  provideRouter(routes, withComponentInputBinding(), withHashLocation()),
  provideSpartanHlm(),
  // Reads the OS before the first render; the remembered Settings → General look wins over it.
  provideSpartanNative({ host: inWails ? wailsHost() : browserHost(), overrides: rememberedLook() }),
  { provide: API, useValue: api },
  // The remembered Settings → General language, else the OS's; the settings set it again once they load.
  { provide: I18N_LANG, useValue: rememberedLang() },
];

/** A plain browser starts in its own light or dark scheme (the screenshots take the dark one this way). */
function browserHost() {
  return staticHost({ scheme: matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light' });
}
