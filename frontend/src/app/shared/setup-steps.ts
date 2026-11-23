import type { ProfileKind, SetupStepDto, SetupStepId } from '../api/api';

/** Where a setup step is done: a profile to add in Settings → Profiles, or another place. */
export type SetupAction = { kind: 'profile'; profile: ProfileKind } | { kind: 'page'; path: string; tab?: string };

/** A setup step ready to show: the backend's step with its link. */
export interface SetupStep {
  /** i18n: `setup.<id>` (title) and `setup.<id>.help`. */
  id: SetupStepId;
  done: boolean;
  action: SetupAction;
}

/**
 * The source page each step belongs to: its "Set up" badge in the side panel. Google and Jira are accounts more than
 * one source uses (Google: chats and calendars; Jira: Tempo), so their steps stay on Settings → Profiles; their
 * source's Setup tab still shows the form. Tempo has no source.
 */
export const SETUP_PANE: Partial<Record<SetupStepId, string>> = {
  repositories: 'repositories',
  calendar: 'meetings',
  hrmaster: 'sessions',
  google: 'chats',
  jira: 'jira',
  confluence: 'confluence',
  email: 'email',
};
const ACCOUNT_STEPS: readonly SetupStepId[] = ['google', 'jira'];

/** The backend's setup steps (Api.setupSteps) with where each is done: its source's Setup tab, else the backend's place. */
export function toSetupSteps(steps: readonly SetupStepDto[]): SetupStep[] {
  return steps.map((s) => {
    const pane = ACCOUNT_STEPS.includes(s.id) ? undefined : SETUP_PANE[s.id];
    return {
      id: s.id,
      done: s.done,
      action: pane
        ? { kind: 'page', path: pane, tab: 'setup' }
        : s.profile
          ? { kind: 'profile', profile: s.profile }
          : { kind: 'page', path: s.page, ...(s.tab ? { tab: s.tab } : {}) },
    };
  });
}

/** The router link of a step. */
export function setupLink(a: SetupAction): { commands: string[]; queryParams: Record<string, string> } {
  if (a.kind === 'profile') return { commands: ['settings'], queryParams: { tab: 'profiles', add: a.profile } };
  return { commands: [a.path], queryParams: a.tab ? { tab: a.tab } : {} };
}
