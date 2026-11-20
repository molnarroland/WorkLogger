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

/** The backend's setup steps (Api.setupSteps) with where each is done. */
export function toSetupSteps(steps: readonly SetupStepDto[]): SetupStep[] {
  return steps.map((s) => ({
    id: s.id,
    done: s.done,
    action: s.profile
      ? { kind: 'profile', profile: s.profile }
      : { kind: 'page', path: s.page, ...(s.tab ? { tab: s.tab } : {}) },
  }));
}

/** The router link of a step. */
export function setupLink(a: SetupAction): { commands: string[]; queryParams: Record<string, string> } {
  if (a.kind === 'profile') return { commands: ['settings'], queryParams: { tab: 'profiles', add: a.profile } };
  return { commands: [a.path], queryParams: a.tab ? { tab: a.tab } : {} };
}
