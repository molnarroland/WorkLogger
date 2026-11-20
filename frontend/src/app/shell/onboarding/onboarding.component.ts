import { ChangeDetectionStrategy, Component, computed, effect, inject, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  lucideActivity,
  lucideBriefcase,
  lucideClock,
  lucideFolderOpen,
  lucideFolderPlus,
  lucideHardDrive,
  lucideMessagesSquare,
  lucideTimer,
  lucideTrash2,
} from '@ng-icons/lucide';
import { SnModalSheet, SnSettingsImports } from '@spartan-native/desktop-ui';
import { NATIVE_HOST } from '@spartan-native/theme';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmSwitchImports } from '@spartan-ng/helm/switch';
import { API, type Module, type ProfileKind } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import { ProfilesSettingsComponent } from '../../pages/settings/profiles-settings.component';
import { ShellStore } from '../../state/shell.store';
import { ChipListComponent } from '../../ui/chip-list/chip-list.component';

/** A question: what it switches on, and the pages a "yes" adds. */
type Answer = 'git' | 'jiraWork' | 'hrmaster' | 'googleChat' | 'jira';
type Page = 'ask' | 'git' | ProfileKind;

const QUESTIONS: { id: Answer; icon: string; module?: Module; pages: Page[] }[] = [
  { id: 'git', icon: 'lucideHardDrive', module: 'git', pages: ['git'] },
  { id: 'jiraWork', icon: 'lucideActivity', module: 'jira', pages: ['jira'] },
  { id: 'hrmaster', icon: 'lucideClock', module: 'workTimes', pages: ['hrmaster'] },
  { id: 'googleChat', icon: 'lucideMessagesSquare', module: 'chats', pages: ['googleChat'] },
  { id: 'jira', icon: 'lucideBriefcase', pages: ['jira', 'tempo'] },
];

/**
 * The first run: what do you use (commits, Jira tickets you plan or analyse, HRmaster, Google Chat, Jira/Tempo), then
 * each "yes" is set up with the same forms as Settings. It stays open until something is set up (a repository folder or a filled profile).
 */
@Component({
  selector: 'wl-onboarding',
  imports: [
    ChipListComponent,
    HlmButtonImports,
    HlmSwitchImports,
    NgIcon,
    ProfilesSettingsComponent,
    SnModalSheet,
    SnSettingsImports,
    TPipe,
  ],
  providers: [
    provideIcons({
      lucideActivity,
      lucideBriefcase,
      lucideClock,
      lucideFolderOpen,
      lucideFolderPlus,
      lucideHardDrive,
      lucideMessagesSquare,
      lucideTimer,
      lucideTrash2,
    }),
  ],
  templateUrl: './onboarding.component.html',
  styleUrl: './onboarding.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class OnboardingComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly host = inject(NATIVE_HOST);

  protected readonly questions = QUESTIONS;
  protected readonly open = signal(true);
  protected readonly answers = signal<Answer[]>([]);
  protected readonly page = signal<Page>('ask');
  protected readonly pages = computed<Page[]>(() => [
    ...new Set<Page>(['ask', ...QUESTIONS.filter((q) => this.answers().includes(q.id)).flatMap((q) => q.pages)]),
  ]);
  protected readonly profileKind = computed(() => {
    const p = this.page();
    return p === 'ask' || p === 'git' ? null : p;
  });
  protected readonly index = computed(() => this.pages().indexOf(this.page()));
  protected readonly last = computed(() => this.index() === this.pages().length - 1);
  /** Something to collect from: the backend's setup steps (calendars aside, they need no account here). */
  protected readonly ready = computed(() => this.store.setup().some((s) => s.done && s.id !== 'calendar'));

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
  }

  protected finish() {
    return this.store.saveSettings({ welcomed: true });
  }

  /** As on the Repositories page: the host's folder picker where it has one, the backend's otherwise. */
  protected async addFolder() {
    let path: string | null = null;
    if (this.host.dialogs) {
      const picked = await this.host.dialogs.pickFolder({});
      if (!picked) return;
      path = picked.path ?? null;
    }
    path ??= (await this.store.attempt(() => this.api.chooseFolder())) ?? null;
    if (path) await this.store.editSettings((api) => api.addWorkspaceRoot(path));
  }

  protected removeFolder(path: string) {
    return this.store.editSettings((api) => api.removeWorkspaceRoot(path));
  }
}
