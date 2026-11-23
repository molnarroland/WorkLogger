import type { Module } from '../api/api';

export type Step = 'setup' | 'gather' | 'fill' | 'finish';

export interface Pane {
  id: string;
  /** i18n key */
  label: string;
  /** Phosphor icon name (outline); the side panel draws its filled version while the page is open (side-icons.ts). */
  icon: string;
  step: Step;
  /** Hidden when this module is switched off. */
  module?: Module;
  /** Where the side panel lists it: the Days tab's top or Month group, the Sources tab's sources or calendar. */
  place: Place;
  /** Shown as a tab of this page instead of in the side panel (Totals under Log in Tempo). */
  parent?: string;
  /** i18n key: its name in the side panel, when that differs from the page's. */
  sideLabel?: string;
}

export type Place = 'view' | 'month' | 'source' | 'calendar';
/** The side panel's two tabs and the places each holds. */
export type SideTab = 'days' | 'sources';
export const TAB_OF: Record<Place, SideTab> = { view: 'days', month: 'days', source: 'sources', calendar: 'sources' };

/**
 * The three views first (Overview, Days, Week: the side panel's top), then the other panes in workflow order
 * (PRODUCT.md). Ctrl+1…9 follow this order.
 */
export const PANES: Pane[] = [
  {
    id: 'overview',
    label: 'pane.overview',
    icon: 'phosphorHouse',
    step: 'fill',
    place: 'view',
  },
  { id: 'days', label: 'pane.days', icon: 'phosphorTray', step: 'fill', place: 'view' },
  {
    id: 'week',
    label: 'pane.week',
    icon: 'phosphorCalendarDots',
    step: 'fill',
    place: 'view',
  },
  {
    id: 'repositories',
    label: 'pane.repositories',
    icon: 'phosphorGitBranch',
    step: 'setup',
    module: 'git',
    place: 'source',
  },
  {
    id: 'meetings',
    label: 'pane.meetings',
    icon: 'phosphorUsers',
    step: 'setup',
    module: 'calendar',
    place: 'source',
  },
  {
    id: 'sessions',
    label: 'pane.sessions',
    icon: 'phosphorClock',
    step: 'gather',
    module: 'workTimes',
    place: 'source',
  },
  {
    id: 'periods',
    label: 'pane.periods',
    icon: 'phosphorFlag',
    step: 'gather',
    place: 'calendar',
  },
  {
    id: 'chats',
    label: 'pane.chats',
    icon: 'phosphorChats',
    step: 'gather',
    module: 'chats',
    place: 'source',
  },
  {
    id: 'plans',
    label: 'pane.plans',
    icon: 'phosphorCalendarPlus',
    step: 'gather',
    place: 'calendar',
  },
  {
    id: 'jira',
    label: 'pane.jira',
    icon: 'phosphorPulse',
    step: 'gather',
    place: 'source',
  },
  {
    id: 'confluence',
    label: 'pane.confluence',
    icon: 'phosphorBookOpen',
    step: 'gather',
    module: 'confluence',
    place: 'source',
  },
  {
    id: 'email',
    label: 'pane.email',
    icon: 'phosphorEnvelopeSimple',
    step: 'gather',
    module: 'email',
    place: 'source',
  },
  {
    id: 'timesheet',
    label: 'pane.timesheet',
    icon: 'phosphorCalendarBlank',
    step: 'fill',
    place: 'month',
  },
  {
    id: 'review',
    label: 'pane.review',
    icon: 'phosphorListChecks',
    step: 'fill',
    place: 'month',
  },
  {
    id: 'by-ticket',
    label: 'pane.byTicket',
    icon: 'phosphorClipboardText',
    step: 'finish',
    place: 'month',
    sideLabel: 'toolbar.logInTempo',
  },
  {
    id: 'totals',
    label: 'pane.totals',
    icon: 'phosphorChartBar',
    step: 'finish',
    place: 'month',
    parent: 'by-ticket',
  },
];

export function visiblePanes(modules: readonly Module[]): Pane[] {
  return PANES.filter((p) => !p.module || modules.includes(p.module));
}
