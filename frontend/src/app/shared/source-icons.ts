import { provideIcons } from '@ng-icons/core';
import {
  lucideActivity,
  lucideBookOpen,
  lucideFlag,
  lucideGitBranch,
  lucideMail,
  lucideMessagesSquare,
  lucidePencil,
  lucideUmbrella,
  lucideUsers,
} from '@ng-icons/lucide';
import type { Source } from '../api/api';

/** One icon per work source. */
export const SOURCE_ICON: Record<Source, string> = {
  git: 'lucideGitBranch',
  jira: 'lucideActivity',
  confluence: 'lucideBookOpen',
  email: 'lucideMail',
  meeting: 'lucideUsers',
  chat: 'lucideMessagesSquare',
  absence: 'lucideUmbrella',
  manual: 'lucidePencil',
  period: 'lucideFlag',
};

export const provideSourceIcons = () =>
  provideIcons({
    lucideActivity,
    lucideBookOpen,
    lucideFlag,
    lucideGitBranch,
    lucideMail,
    lucideMessagesSquare,
    lucidePencil,
    lucideUmbrella,
    lucideUsers,
  });
