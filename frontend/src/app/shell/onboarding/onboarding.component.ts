import { ChangeDetectionStrategy, Component, computed, effect, inject, resource, signal } from '@angular/core';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  phosphorPulse,
  phosphorBriefcase,
  phosphorArrowCircleDown,
  phosphorCheckCircle,
  phosphorCircleDashed,
  phosphorClipboardText,
  phosphorClock,
  phosphorHardDrive,
  phosphorTray,
  phosphorChats,
} from '@ng-icons/phosphor-icons/regular';
import { SnModalSheet, SnSettingsImports } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import { HlmSwitchImports } from '@spartan-ng/helm/switch';
import { API, type Module, type ProfileKind } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import { ProfilesSettingsComponent } from '../../pages/settings/profiles-settings.component';
import { ShellStore } from '../../state/shell.store';
import { RepoSetupComponent } from '../../pages/repositories/repo-setup.component';
import { monthTitle } from '../../shared/month';
import { I18n } from '../../i18n/t.pipe';
import { PANES } from '../panes';

/** A question: what it switches on, and the pages a "yes" adds. */
type Answer = 'git' | 'jiraWork' | 'hrmaster' | 'googleChat' | 'jira';
type Page = 'welcome' | 'ask' | 'git' | ProfileKind | 'collect';

/** The month's three steps, on the welcome page. */
const MONTH_STEPS = [
  { id: 'collect', icon: 'phosphorArrowCircleDown' },
  { id: 'check', icon: 'phosphorTray' },
  { id: 'log', icon: 'phosphorClipboardText' },
];

const QUESTIONS: { id: Answer; icon: string; module?: Module; pages: Page[] }[] = [
  { id: 'git', icon: 'phosphorHardDrive', module: 'git', pages: ['git'] },
  { id: 'jiraWork', icon: 'phosphorPulse', module: 'jira', pages: ['jira'] },
  { id: 'hrmaster', icon: 'phosphorClock', module: 'workTimes', pages: ['hrmaster'] },
  { id: 'googleChat', icon: 'phosphorChats', module: 'chats', pages: ['googleChat'] },
  { id: 'jira', icon: 'phosphorBriefcase', pages: ['jira', 'tempo'] },
];

/**
 * The first run (docs/LAYOUT-PLAN.md, phase 3): what the month looks like, what do you use (commits, Jira tickets you
 * plan or analyse, HRmaster, Google Chat, Jira/Tempo), then each "yes" is set up with the same forms as its source's
 * Setup tab (an account can be skipped: it stays on Sources → Finish setup), and last the first Collect. It stays
 * open until something is set up (a repository folder or a filled profile).
 */
@Component({
  selector: 'wl-onboarding',
  imports: [
    HlmButtonImports,
    HlmSpinnerImports,
    HlmSwitchImports,
    NgIcon,
    ProfilesSettingsComponent,
    RepoSetupComponent,
    SnModalSheet,
    SnSettingsImports,
    TPipe,
  ],
  providers: [
    provideIcons({
      phosphorPulse,
      phosphorBriefcase,
      phosphorArrowCircleDown,
      phosphorCheckCircle,
      phosphorCircleDashed,
      phosphorClipboardText,
      phosphorClock,
      phosphorHardDrive,
      phosphorTray,
      phosphorChats,
    }),
  ],
  templateUrl: './onboarding.component.html',
  styleUrl: './onboarding.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OnboardingComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly router = inject(Router);
  private readonly i18n = inject(I18n);

  protected readonly questions = QUESTIONS;
  protected readonly monthSteps = MONTH_STEPS;
  protected readonly open = signal(true);
  protected readonly answers = signal<Answer[]>([]);
  protected readonly page = signal<Page>('welcome');
  protected readonly pages = computed<Page[]>(() => [
    ...new Set<Page>([
      'welcome',
      'ask',
      ...QUESTIONS.filter((q) => this.answers().includes(q.id)).flatMap((q) => q.pages),
      'collect',
    ]),
  ]);
  protected readonly profileKind = computed(() => {
    const p = this.page();
    return p === 'welcome' || p === 'ask' || p === 'git' || p === 'collect' ? null : p;
  });
  protected readonly index = computed(() => this.pages().indexOf(this.page()));
  protected readonly last = computed(() => this.index() === this.pages().length - 1);
  /** Something to collect from: the backend's setup steps (calendars aside, they need no account here). */
  protected readonly ready = computed(() => this.store.setup().some((s) => s.done && s.id !== 'calendar'));
  protected readonly monthTitle = computed(() => monthTitle(this.store.month(), this.i18n.lang()));
  /** The Collect page: the job this sheet started has ended (the store's busy slot is free again). */
  protected readonly collected = signal(false);
  /** The last task, when it is this sheet's Collect and it failed. */
  protected readonly collectError = computed(() => {
    const t = this.store.tasks().at(-1);
    return this.collected() && t?.state === 'failed' ? t.message : null;
  });
  /** What Collect brought in for each source that is on, re-read with the month. */
  private readonly counts = resource({
    params: () => {
      const { from, to } = this.store.range();
      return this.page() === 'collect' && from ? { from, to, data: this.store.data() } : undefined;
    },
    loader: ({ params }) => this.api.sourceCounts(params.from, params.to),
  });
  protected readonly sourceRows = computed(() => {
    const counts = this.counts.value();
    const on = this.store.paneIds();
    return PANES.filter(
      (p) => p.place === 'source' && on.includes(p.id) && (p.id !== 'jira' || this.store.modules().includes('jira')),
    ).map((p) => ({ id: p.id, label: p.label, count: counts?.[p.id as keyof typeof counts] ?? null }));
  });
  /** The setup steps left: they stay on Sources → Finish setup. */
  protected readonly stepsLeft = computed(() => this.store.setup().filter((s) => !s.done));

  constructor() {
    // Esc closes a sheet; this one stays until it is done.
    effect(() => {
      if (!this.open()) this.open.set(true);
    });
  }

  protected answer(id: Answer, yes: boolean) {
    const rest = this.answers().filter((a) => a !== id);
    this.answers.set(yes ? [...rest, id] : rest);
  }

  /** Leaving the questions saves them: the modules asked about, and Tempo by hand unless Jira/Tempo is used. */
  protected async go(by: number) {
    if (this.page() === 'ask' && by > 0) {
      const s = this.store.settings();
      if (!s) return;
      const asked = QUESTIONS.flatMap((q) => (q.module ? [q.module] : []));
      const on = QUESTIONS.filter((q) => q.module && this.answers().includes(q.id)).map((q) => q.module!);
      await this.store.saveSettings({
        modules: [...s.modules.filter((m) => !asked.includes(m)), ...on],
        tempoByHand: !this.answers().includes('jira'),
      });
    }
    this.page.set(this.pages()[this.index() + by]);
    if (this.page() === 'collect') void this.collect();
  }

  /** The first Collect of the month on screen. */
  protected async collect() {
    if (!this.ready()) return;
    this.collected.set(false);
    await this.store.run('collect');
    this.collected.set(true);
  }

  /** Done: the sheet goes, and the first day that needs work opens (the overview when there is none). */
  protected async finish() {
    await this.store.saveSettings({ welcomed: true });
    const day = this.store.attention()[0]?.day;
    await this.router.navigate(day ? ['/days', day] : ['/overview']);
  }
}
