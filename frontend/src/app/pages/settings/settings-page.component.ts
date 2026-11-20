import { ChangeDetectionStrategy, Component, computed, inject, input, linkedSignal, resource } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  lucideActivity,
  lucideAppWindow,
  lucideBlend,
  lucideBookOpen,
  lucideCalendarClock,
  lucideClock,
  lucideContrast,
  lucideDatabase,
  lucideHardDrive,
  lucideLanguages,
  lucideMail,
  lucideMessagesSquare,
  lucidePalette,
  lucideSunMoon,
  lucideType,
  lucideUsers,
  lucideWind,
} from '@ng-icons/lucide';
import { SnNumberBox, SnSettingsImports } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { HlmLabelImports } from '@spartan-ng/helm/label';
import { HlmSelectImports } from '@spartan-ng/helm/select';
import { HlmSwitchImports } from '@spartan-ng/helm/switch';
import { HlmTabsImports } from '@spartan-ng/helm/tabs';
import { HlmToggleGroupImports } from '@spartan-ng/helm/toggle-group';
import { API, type Module, type ProfileKind, type SettingsDto } from '../../api/api';
import { I18n, LANGUAGES, type LanguageChoice, TPipe } from '../../i18n/t.pipe';
import {
  ACCENTS,
  ACCENT_HEX,
  APPEARANCES,
  CONTRASTS,
  MOTIONS,
  SKINS,
  TEXT_SCALE_CHOICES,
  TRANSPARENCIES,
  type AccentColor,
  type Appearance,
  type Look,
  lookOf,
} from '../../shared/appearance';
import { ShellStore } from '../../state/shell.store';
import { ChipListComponent } from '../../ui/chip-list/chip-list.component';
import { type RuleItem, RuleListComponent } from '../../ui/rule-list/rule-list.component';
import { CalendarSettingsComponent } from './calendar-settings.component';
import { ProfilesSettingsComponent } from './profiles-settings.component';
import { MODULES } from '../../ui/welcome-sheet/welcome-sheet.component';

export type SettingsTab = 'general' | 'modules' | 'profiles' | 'calendars' | 'timesheet' | 'coding' | 'chats';

/** Settings, one page with tabs. Every change saves straight away. */
@Component({
  selector: 'wl-settings-page',
  imports: [
    CalendarSettingsComponent,
    ChipListComponent,
    HlmButtonImports,
    HlmInputImports,
    HlmLabelImports,
    HlmSelectImports,
    HlmSwitchImports,
    HlmTabsImports,
    HlmToggleGroupImports,
    NgIcon,
    ProfilesSettingsComponent,
    RuleListComponent,
    SnNumberBox,
    SnSettingsImports,
    TPipe,
  ],
  providers: [
    provideIcons({
      lucideActivity,
      lucideAppWindow,
      lucideBookOpen,
      lucideBlend,
      lucideCalendarClock,
      lucideClock,
      lucideContrast,
      lucideDatabase,
      lucideHardDrive,
      lucideLanguages,
      lucideMail,
      lucideMessagesSquare,
      lucidePalette,
      lucideSunMoon,
      lucideType,
      lucideUsers,
      lucideWind,
    }),
  ],
  templateUrl: './settings-page.component.html',
  styleUrl: './settings-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SettingsPageComponent {
  protected readonly store = inject(ShellStore);
  protected readonly api = inject(API);
  private readonly i18n = inject(I18n);
  protected readonly tabs: SettingsTab[] = [
    'general',
    'modules',
    'profiles',
    'calendars',
    'timesheet',
    'coding',
    'chats',
  ];
  protected readonly languages = LANGUAGES;
  protected readonly appearances = APPEARANCES;
  protected readonly accents = ACCENTS;
  protected readonly accentHex = ACCENT_HEX as Record<string, string>;
  protected readonly skins = SKINS;
  protected readonly transparencies = TRANSPARENCIES;
  protected readonly contrasts = CONTRASTS;
  protected readonly motions = MOTIONS;
  protected readonly textScales = TEXT_SCALE_CHOICES;
  /** The saved look, with anything unknown read as "system". */
  protected readonly look = computed(() => lookOf(this.store.settings() ?? {}));
  protected readonly roundUps = [15, 30, 60];
  protected readonly granularities = [5, 10, 15, 30, 60];

  /** `?tab=…` opens a tab. The links from before profiles (`jira`, `sites`) open Profiles. */
  // eslint-disable-next-line @angular-eslint/no-input-rename -- the alias is the URL query parameter name
  readonly initialTab = input<string | undefined>(undefined, { alias: 'tab' });
  /** `?add=hrmaster` (from the setup guide): open that kind of profile on Profiles, adding one if there is none. */
  // eslint-disable-next-line @angular-eslint/no-input-rename -- the alias is the URL query parameter name
  readonly addKind = input<string | undefined>(undefined, { alias: 'add' });
  protected readonly addProfile = computed<ProfileKind | null>(() => {
    const k = this.addKind();
    return (['googleChat', 'hrmaster', 'jira', 'tempo', 'confluence', 'email'] as string[]).includes(k ?? '')
      ? (k as ProfileKind)
      : null;
  });
  protected readonly tab = linkedSignal<SettingsTab>(() => {
    const t = this.initialTab();
    if (t === 'jira' || t === 'sites') return 'profiles';
    return (this.tabs as string[]).includes(t ?? '') ? (t as SettingsTab) : 'general';
  });
  /** Welcome → "Continue to Tempo settings" opens the first Jira profile. */
  protected readonly focusKind = computed<ProfileKind | null>(() => (this.initialTab() === 'jira' ? 'jira' : null));

  protected readonly dbPath = resource({ loader: () => this.api.databasePath() });
  protected readonly absenceRules = computed<RuleItem[]>(() =>
    (this.store.settings()?.absenceRules ?? []).map((r) => ({ label: r.contains, issueKey: r.issueKey })),
  );
  protected readonly chatRules = computed<RuleItem[]>(() =>
    (this.store.settings()?.chatRules ?? []).map((r) => ({ label: r.name, issueKey: r.issueKey })),
  );

  protected readonly roundingLabel = (v: unknown) =>
    this.i18n.t(v === 'estimate' ? 'settings.rounding.estimate' : 'settings.rounding.fixedHours');
  protected readonly roundUpLabel = (v: unknown) =>
    v === 60 ? this.i18n.t('settings.wholeHours') : this.i18n.t('settings.minutes', { n: Number(v) });
  protected readonly minutesLabel = (v: unknown) => this.i18n.t('settings.minutes', { n: Number(v) });
  protected readonly skinLabel = (v: unknown) => this.i18n.t('settings.skin.' + String(v));
  protected readonly textScaleLabel = (v: unknown) =>
    Number(v) ? `${Math.round(Number(v) * 100)} %` : this.i18n.t('settings.textScale.system');

  protected readonly modules = MODULES;

  /** A data source on or off. A page whose module goes off leaves the sidebar; this page stays. */
  protected toggleModule(id: Module, on: boolean) {
    const rest = this.store.modules().filter((m) => m !== id);
    return this.save({ modules: on ? [...rest, id] : rest });
  }

  protected setTab(t: string) {
    if ((this.tabs as string[]).includes(t)) this.tab.set(t as SettingsTab);
  }

  protected save(patch: Partial<SettingsDto>) {
    return this.store.saveSettings(patch);
  }

  /** Number boxes: clamped to their ranges; an empty box keeps the old value. */
  protected saveNumber(key: keyof SettingsDto, value: number | null, min: number, max: number) {
    if (value === null || Number.isNaN(value)) return;
    const v = Math.min(max, Math.max(min, value));
    if (v !== this.store.settings()?.[key]) return this.save({ [key]: v });
    return;
  }

  protected setLanguage(v: unknown) {
    if (LANGUAGES.includes(v as LanguageChoice)) return this.save({ language: v as LanguageChoice });
    return;
  }

  protected setAppearance(v: unknown) {
    if (APPEARANCES.includes(v as Appearance)) return this.save({ appearance: v as Appearance });
    return;
  }

  protected setAccent(v: unknown) {
    if (ACCENTS.includes(v as AccentColor)) return this.save({ accentColor: v as AccentColor });
    return;
  }

  /** Skin, transparency, contrast, motion, text size: one of the listed choices, saved as it is. */
  protected setLook<K extends Exclude<keyof Look, 'appearance' | 'accent'>>(key: K, v: unknown) {
    const allowed: Record<typeof key, readonly unknown[]> = {
      skin: SKINS,
      transparency: TRANSPARENCIES,
      contrast: CONTRASTS,
      motion: MOTIONS,
      textScale: TEXT_SCALE_CHOICES,
    } as never;
    const value = key === 'textScale' ? Number(v) : v;
    if (allowed[key].includes(value) && value !== this.look()[key]) return this.save({ [key]: value });
    return;
  }

  protected setRounding(v: unknown) {
    if (v === 'estimate' || v === 'fixedHours') return this.save({ roundingMode: v });
    return;
  }

  protected value(e: Event) {
    return (e.target as HTMLInputElement).value;
  }

  protected addAbsenceRule(r: RuleItem) {
    return this.store.editSettings((api) => api.addAbsenceRule(r.label, r.issueKey ?? ''));
  }

  protected removeAbsenceRule(i: number) {
    const id = this.store.settings()?.absenceRules[i]?.id;
    if (id) return this.store.editSettings((api) => api.removeAbsenceRule(id));
    return;
  }

  protected removeChatRule(i: number) {
    const r = this.store.settings()?.chatRules[i];
    if (r) return this.store.editSettings((api) => api.setChatRule(r.conversationId, r.name, ''));
    return;
  }
}
