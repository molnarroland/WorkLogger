import { provideIcons } from '@ng-icons/core';
import {
  phosphorPulse,
  phosphorBookOpen,
  phosphorFlag,
  phosphorGitBranch,
  phosphorEnvelopeSimple,
  phosphorChats,
  phosphorPencilSimple,
  phosphorUmbrella,
  phosphorUsers,
} from '@ng-icons/phosphor-icons/regular';
import type { Source } from '../api/api';

/** One icon per work source. */
export const SOURCE_ICON: Record<Source, string> = {
  git: 'phosphorGitBranch',
  jira: 'phosphorPulse',
  confluence: 'phosphorBookOpen',
  email: 'phosphorEnvelopeSimple',
  meeting: 'phosphorUsers',
  chat: 'phosphorChats',
  absence: 'phosphorUmbrella',
  manual: 'phosphorPencilSimple',
  period: 'phosphorFlag',
};

export const provideSourceIcons = () =>
  provideIcons({
    phosphorPulse,
    phosphorBookOpen,
    phosphorFlag,
    phosphorGitBranch,
    phosphorEnvelopeSimple,
    phosphorChats,
    phosphorPencilSimple,
    phosphorUmbrella,
    phosphorUsers,
  });
