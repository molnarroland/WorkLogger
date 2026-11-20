import type { SnMenuItem } from '@spartan-native/desktop-ui';
import type { Module } from '../api/api';
import type { Job } from '../state/shell.store';
import type { Pane } from './panes';
import { ACCELERATORS } from './shortcuts';

/** What the menu shows and what its items do; the shell supplies both. */
export interface AppMenuInput {
  locked: boolean;
  busy: boolean;
  modules: readonly Module[];
  panes: readonly Pane[];
  isCurrentMonth: boolean;
  t: (key: string) => string;
  run: (job: Job) => void;
  go: (pane: string) => void;
  shiftMonth: (by: number) => void;
  thisMonth: () => void;
  openExport: () => void;
  openGuide: () => void;
  openModules: () => void;
}

/** The macOS menu bar: the same commands as the command bar, with their shortcuts. */
export function appMenu(m: AppMenuInput): SnMenuItem[] {
  // Locked: only the system's own menus; nothing may reach the backend before the keyring is read.
  if (m.locked) return [{ role: 'appMenu' }, { role: 'editMenu' }, { role: 'windowMenu' }];
  const { t } = m;
  const job = (label: string, kind: Job, accelerator?: string): SnMenuItem => ({
    label: t(label),
    accelerator,
    enabled: !m.busy,
    action: () => m.run(kind),
  });
  return [
    { role: 'appMenu' },
    {
      label: t('menu.file'),
      submenu: [
        job('toolbar.collectRange', 'collect', ACCELERATORS.collect),
        job('toolbar.rebuild', 'rebuild', ACCELERATORS.rebuild),
        { type: 'separator' },
        ...(m.modules.includes('workTimes') ? [job('toolbar.fetchHrmaster', 'fetchHrmaster')] : []),
        ...(m.modules.includes('chats') ? [job('toolbar.fetchChat', 'fetchGoogleChat')] : []),
        { type: 'separator' },
        { label: t('toolbar.export') + '…', action: m.openExport },
        { label: t('toolbar.guide') + '…', action: m.openGuide },
        { label: t('toolbar.modules'), action: m.openModules },
        { type: 'separator' },
        { role: 'close' },
      ],
    },
    { role: 'editMenu' },
    {
      label: t('menu.go'),
      submenu: [
        ...m.panes.map((p, i): SnMenuItem => ({
          label: t(p.label),
          accelerator: i < 9 ? ACCELERATORS.page(i) : undefined,
          action: () => m.go(p.id),
        })),
        { type: 'separator' },
        { label: t('month.previous'), accelerator: ACCELERATORS.prevMonth, action: () => m.shiftMonth(-1) },
        { label: t('month.next'), accelerator: ACCELERATORS.nextMonth, action: () => m.shiftMonth(1) },
        { label: t('month.this'), enabled: !m.isCurrentMonth, action: m.thisMonth },
        { type: 'separator' },
        { label: t('toolbar.logInTempo'), accelerator: ACCELERATORS.logInTempo, action: () => m.go('by-ticket') },
        { label: t('toolbar.settings'), action: () => m.go('settings') },
      ],
    },
    { role: 'windowMenu' },
  ];
}
