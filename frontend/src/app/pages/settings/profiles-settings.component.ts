import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  input,
  linkedSignal,
  resource,
  signal,
  untracked,
} from '@angular/core';
import { Router } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  lucideBookOpen,
  lucideBriefcase,
  lucideClock,
  lucideMail,
  lucideMessagesSquare,
  lucidePlus,
  lucideTimer,
  lucideTrash2,
} from '@ng-icons/lucide';
import { SnSettingsImports } from '@spartan-native/desktop-ui';
import { SnSelectableImports } from '@spartan-native/helm';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmDropdownMenuImports } from '@spartan-ng/helm/dropdown-menu';
import { API, type CheckLine, type JiraAccount, type Profile, type ProfileKind, type TokenKind } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { ShellStore } from '../../state/shell.store';
import { JiraAccountFormComponent } from '../../ui/jira-account-form/jira-account-form.component';
import { type JiraChoice, ProfileEditorComponent } from '../../ui/profile-editor/profile-editor.component';
import { SignInBadgeComponent } from '../../ui/sign-in-badge/sign-in-badge.component';
import { SetupChecklistComponent } from '../../ui/setup-checklist/setup-checklist.component';
import { type SetupStep, setupLink, toSetupSteps } from '../../shared/setup-steps';

export const PROFILE_KINDS: ProfileKind[] = ['googleChat', 'hrmaster', 'jira', 'tempo', 'confluence', 'email'];
export const PROFILE_ICON: Record<ProfileKind, string> = {
  googleChat: 'lucideMessagesSquare',
  hrmaster: 'lucideClock',
  jira: 'lucideBriefcase',
  tempo: 'lucideTimer',
  confluence: 'lucideBookOpen',
  email: 'lucideMail',
};

interface Item {
  key: string;
  kind: ProfileKind;
  title: string;
  detail: string;
  profile: Profile | null;
  account: JiraAccount | null;
}

/**
 * Settings → Profiles: every account WorkLogger reads from or logs to. Google (Chat and Calendar), HRmaster
 * (work sessions), Jira (issues and worklogs) and Tempo (for one Jira profile). Several of a kind are all read.
 */
@Component({
  selector: 'wl-profiles-settings',
  imports: [
    HlmBadgeImports,
    HlmButtonImports,
    HlmDropdownMenuImports,
    JiraAccountFormComponent,
    NgIcon,
    ProfileEditorComponent,
    SetupChecklistComponent,
    SignInBadgeComponent,
    SnSelectableImports,
    SnSettingsImports,
    TPipe,
  ],
  providers: [
    provideIcons({
      lucideBookOpen,
      lucideBriefcase,
      lucideClock,
      lucideMail,
      lucideMessagesSquare,
      lucidePlus,
      lucideTimer,
      lucideTrash2,
    }),
  ],
  templateUrl: './profiles-settings.component.html',
  styleUrl: './profiles-settings.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProfilesSettingsComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);
  protected readonly kinds = PROFILE_KINDS;
  protected readonly icon = PROFILE_ICON;

  /** A profile to open first, e.g. "jira" after the welcome sheet: the first profile of that kind. */
  readonly focusKind = input<ProfileKind | null>(null);
  /** From the setup guide: open the first profile of this kind, or add one when there is none. */
  readonly addKind = input<ProfileKind | null>(null);
  /** The onboarding: only this kind's form (its first profile, added when there is none), no list or checklist. */
  readonly only = input<ProfileKind | null>(null);
  private readonly router = inject(Router);
  protected readonly checkLines = signal<CheckLine[]>([]);
  protected readonly checking = signal(false);
  protected readonly loadingProjects = signal(false);

  private readonly settings = computed(() => this.store.settings());
  private readonly loaded = computed(() => !!this.settings());
  /** What the chosen modules still need; shown above the list until all of it is done. */
  protected readonly setup = computed(() => toSetupSteps(this.store.setup()));
  protected readonly setupLeft = computed(() => this.setup().some((s) => !s.done));
  protected readonly items = computed<Item[]>(() => {
    const s = this.settings();
    if (!s) return [];
    const out: Item[] = [];
    for (const p of s.profiles) {
      const linked = p.kind === 'tempo' || p.kind === 'confluence';
      const account = linked ? (s.jiraAccounts.find((a) => a.id === p.jiraAccountId) ?? null) : null;
      out.push({
        key: 'p:' + p.id,
        kind: p.kind,
        title: p.name || p.email || this.host(p.url) || this.i18n.t('profiles.kind.' + p.kind),
        detail: linked
          ? account
            ? this.accountTitle(account)
            : this.host(p.url)
          : p.kind === 'hrmaster' || p.kind === 'email'
            ? this.host(p.url)
            : p.email,
        profile: p,
        account: null,
      });
    }
    for (const a of s.jiraAccounts) {
      out.push({
        key: 'j:' + a.id,
        kind: 'jira',
        title: this.accountTitle(a),
        detail: a.projectKeys.join(', '),
        profile: null,
        account: a,
      });
    }
    return out.sort((x, y) => this.kinds.indexOf(x.kind) - this.kinds.indexOf(y.kind));
  });

  protected readonly selected = linkedSignal<string | null>(() => {
    const kind = this.focusKind();
    return (kind ? this.items().find((i) => i.kind === kind)?.key : null) ?? null;
  });
  protected readonly current = computed(() => {
    const items = this.items();
    return items.find((i) => i.key === this.selected()) ?? items[0] ?? null;
  });
  protected readonly shownSelection = computed<readonly string[]>(() => (this.current() ? [this.current()!.key] : []));

  /** The backend keeps it on an existing Jira profile. */
  protected readonly defaultId = computed(() => this.settings()?.defaultJiraAccountId ?? null);
  /** Set when "Add Tempo" found every Jira profile linked already (the backend's answer). */
  protected readonly tempoNeedsJira = signal(false);

  /** The sign-in check of every Google and HRmaster profile's window. */
  protected readonly signIns = resource({
    params: () =>
      (this.settings()?.profiles ?? [])
        .filter((p) => p.kind === 'googleChat' || p.kind === 'hrmaster')
        .map((p) => p.id),
    loader: async ({ params }) => {
      const out: Record<string, boolean | null> = {};
      for (const id of params) out[id] = await this.api.signedIn(id);
      return out;
    },
  });
  /** Whether the open profile's token is saved: the Jira token, or the linked Jira account's Tempo token. */
  protected readonly tokens = resource({
    params: () => {
      const c = this.current();
      if (c?.account) return { accountId: c.account.id, kind: 'jira' as const };
      if (c?.profile?.kind === 'tempo' && c.profile.jiraAccountId)
        return { accountId: c.profile.jiraAccountId, kind: 'tempo' as const };
      if (c?.profile?.kind === 'confluence' || c?.profile?.kind === 'email')
        return { accountId: c.profile.id, kind: c.profile.kind };
      return undefined;
    },
    loader: async ({ params }) => ({
      kind: params.kind,
      saved: await this.api.hasToken(params.accountId, params.kind),
    }),
  });

  constructor() {
    effect(() => {
      const kind = this.only();
      if (kind && this.loaded()) untracked(() => this.openKind(kind));
    });
    effect(() => {
      const kind = this.addKind();
      if (!kind || !this.settings()) return;
      untracked(() => this.openKind(kind));
      // Done: drop ?add, so going back or reloading doesn't add another.
      this.router.navigate([], { queryParams: { add: null }, queryParamsHandling: 'merge', replaceUrl: true });
    });
  }

  /** A setup step's button: a profile is opened (or added) here; anything else is another page. */
  protected act(step: SetupStep) {
    if (step.action.kind === 'profile') return this.openKind(step.action.profile);
    const { commands, queryParams } = setupLink(step.action);
    return this.router.navigate(commands, { queryParams });
  }

  /** The first profile of a kind, or a new one when there is none. */
  private openKind(kind: ProfileKind) {
    const existing = this.items().find((i) => i.kind === kind);
    if (existing) return this.select([existing.key]);
    return this.add(kind);
  }

  protected accountTitle(a: JiraAccount) {
    return a.name || this.host(a.baseURL) || this.i18n.t('jira.newAccount');
  }

  protected host(url: string) {
    try {
      return url ? new URL(url).host : '';
    } catch {
      return url;
    }
  }

  protected jiraChoices(p: Profile): JiraChoice[] {
    const s = this.settings();
    if (!s) return [];
    const linked = new Set(
      p.kind === 'tempo'
        ? s.profiles.filter((x) => x.kind === 'tempo' && x.id !== p.id).map((x) => x.jiraAccountId)
        : [],
    );
    return s.jiraAccounts.filter((a) => !linked.has(a.id)).map((a) => ({ id: a.id, title: this.accountTitle(a) }));
  }

  /** Where a profile's token or password is kept: Tempo's under its Jira profile, the others' under their own id. */
  protected secretOf(p: Profile): { kind: TokenKind; id: string } {
    return p.kind === 'tempo' ? { kind: 'tempo', id: p.jiraAccountId } : { kind: p.kind as TokenKind, id: p.id };
  }

  /** The Tempo profile linking a Jira account, by title. */
  protected tempoFor(a: JiraAccount): string | null {
    const t = this.settings()?.profiles.find((p) => p.kind === 'tempo' && p.jiraAccountId === a.id);
    return t ? t.name || this.i18n.t('profiles.kind.tempo') : null;
  }

  protected select(keys: readonly string[]) {
    if (!keys.length) return;
    this.selected.set(keys[0]);
    this.checkLines.set([]);
  }

  /**
   * A new profile, made by the backend. A Tempo profile needs a Jira profile without one: then that Jira profile
   * (or a new one) opens instead, with the hint.
   */
  protected async add(kind: ProfileKind): Promise<void> {
    try {
      const e = await this.api.addProfile(kind);
      this.tempoNeedsJira.set(e.needsJira);
      if (e.needsJira) {
        const jira = this.items().find((i) => i.kind === 'jira');
        return jira ? this.select([jira.key]) : this.add('jira');
      }
      this.store.settings.set(e.settings);
      this.store.setup.set(await this.api.setupSteps());
      this.selected.set((kind === 'jira' ? 'j:' : 'p:') + e.id);
    } catch (err) {
      this.store.report(err);
    }
    this.checkLines.set([]);
  }

  /** The backend deletes its tokens too (and, for a Jira profile, the Tempo profile linking it). */
  protected async remove(item: Item) {
    const id = item.account?.id ?? item.profile?.id;
    if (!id) return;
    await this.store.editSettings((api) => api.removeProfile(id));
    this.selected.set(null);
    this.checkLines.set([]);
  }

  /** A Tempo profile moving to another Jira profile takes its token along (in the backend). */
  protected async updateProfile(p: Profile) {
    await this.store.editSettings((api) => api.updateProfile(p));
    this.signIns.reload();
    this.tokens.reload();
  }

  protected updateAccount(a: JiraAccount) {
    return this.store.editSettings((api) => api.updateJiraAccount(a));
  }

  protected makeDefault(a: JiraAccount) {
    return this.store.editSettings((api) => api.setDefaultJiraAccount(a.id));
  }

  /** Saves a new token (the field never shows the old one). */
  protected async setToken(kind: TokenKind, accountId: string, value: string) {
    try {
      await this.api.setToken(accountId, kind, value);
      // The backend forgot what it read with the old token (and a Jira token's account id): read it again now.
      this.store.settings.set(await this.api.settings());
      void this.store.reload().then(() => this.store.refreshTempo());
    } catch (e) {
      this.store.report(e);
    }
    this.tokens.reload();
  }

  protected async signIn(p: Profile) {
    try {
      await this.api.openSite(p.id);
    } catch (e) {
      this.store.report(e);
    }
    this.signIns.reload();
  }

  protected async signOut(p: Profile) {
    await this.store.attempt(() => this.api.signOut(p.id));
    this.signIns.reload();
  }

  protected async readProjects(id: string) {
    this.loadingProjects.set(true);
    try {
      // readProjects saves them into the profile; the settings are read again to show them.
      if ((await this.store.attempt(() => this.api.readProjects(id))) !== undefined) {
        await this.store.editSettings((api) => api.settings());
      }
    } finally {
      this.loadingProjects.set(false);
    }
  }

  protected async check(accountId: string, kind: TokenKind) {
    this.checking.set(true);
    try {
      const lines = await this.store.attempt(async () => {
        if (kind === 'confluence') return this.api.checkConfluence(accountId);
        if (kind === 'email') return this.api.checkEmail(accountId);
        const jira = await this.api.checkJira(accountId);
        return kind === 'tempo' ? [...jira, ...(await this.api.checkTempo(accountId))] : jira;
      });
      if (lines) this.checkLines.set(lines);
    } finally {
      this.checking.set(false);
    }
  }
}
