import type { Module } from '../api/api';

export type Step = 'setup' | 'gather' | 'fill' | 'finish';

export interface Pane {
  id: string;
  /** i18n key */
  label: string;
  /** lucide icon name, registered in sidebar-item */
  icon: string;
  step: Step;
  /** Hidden when this module is switched off. */
  module?: Module;
  /** One of the three views at the top of the side panel; the other panes are under "All pages". */
  view?: boolean;
}

/**
 * The three views first (Overview, Days, Week: the side panel's top), then the other panes in workflow order
 * (PRODUCT.md), which "All pages" lists by step. Ctrl+1…9 follow this order.
 */
export const PANES: Pane[] = [
  { id: 'overview', label: 'pane.overview', icon: 'lucideHouse', step: 'fill', view: true },
  { id: 'days', label: 'pane.days', icon: 'lucideInbox', step: 'fill', view: true },
  { id: 'week', label: 'pane.week', icon: 'lucideCalendarRange', step: 'fill', view: true },
  { id: 'repositories', label: 'pane.repositories', icon: 'lucideHardDrive', step: 'setup', module: 'git' },
  { id: 'meetings', label: 'pane.meetings', icon: 'lucideUsers', step: 'setup', module: 'calendar' },
  { id: 'sessions', label: 'pane.sessions', icon: 'lucideClock', step: 'gather', module: 'workTimes' },
  { id: 'periods', label: 'pane.periods', icon: 'lucideFlag', step: 'gather' },
  { id: 'chats', label: 'pane.chats', icon: 'lucideMessagesSquare', step: 'gather', module: 'chats' },
  { id: 'plans', label: 'pane.plans', icon: 'lucideCalendarClock', step: 'gather' },
  { id: 'jira', label: 'pane.jira', icon: 'lucideActivity', step: 'gather' },
  { id: 'confluence', label: 'pane.confluence', icon: 'lucideBookOpen', step: 'gather', module: 'confluence' },
  { id: 'email', label: 'pane.email', icon: 'lucideMail', step: 'gather', module: 'email' },
  { id: 'timesheet', label: 'pane.timesheet', icon: 'lucideCalendar', step: 'fill' },
  { id: 'review', label: 'pane.review', icon: 'lucideListChecks', step: 'fill' },
  { id: 'by-ticket', label: 'pane.byTicket', icon: 'lucideClipboardList', step: 'finish' },
  { id: 'totals', label: 'pane.totals', icon: 'lucideChartBar', step: 'finish' },
];

export const STEPS: Step[] = ['setup', 'gather', 'fill', 'finish'];

export function visiblePanes(modules: readonly Module[]): Pane[] {
  return PANES.filter((p) => !p.module || modules.includes(p.module));
}
