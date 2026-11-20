import type { Type } from '@angular/core';
import { Routes } from '@angular/router';
import { PANES } from './shell/panes';

/** Each page loads on first visit. */
const PAGES: Record<string, () => Promise<Type<unknown>>> = {
  overview: () => import('./pages/overview/overview-page.component').then((m) => m.OverviewPageComponent),
  days: () => import('./pages/days/days-page.component').then((m) => m.DaysPageComponent),
  week: () => import('./pages/week/week-page.component').then((m) => m.WeekPageComponent),
  timesheet: () => import('./pages/timesheet/timesheet-page.component').then((m) => m.TimesheetPageComponent),
  review: () => import('./pages/review/review-page.component').then((m) => m.ReviewPageComponent),
  'by-ticket': () => import('./pages/by-ticket/by-ticket-page.component').then((m) => m.ByTicketPageComponent),
  totals: () => import('./pages/totals/totals-page.component').then((m) => m.TotalsPageComponent),
  sessions: () => import('./pages/sessions/sessions-page.component').then((m) => m.SessionsPageComponent),
  meetings: () => import('./pages/meetings/meetings-page.component').then((m) => m.MeetingsPageComponent),
  chats: () => import('./pages/chats/chats-page.component').then((m) => m.ChatsPageComponent),
  plans: () => import('./pages/plans/plans-page.component').then((m) => m.PlansPageComponent),
  jira: () => import('./pages/jira-activity/jira-activity-page.component').then((m) => m.JiraActivityPageComponent),
  email: () => import('./pages/email/email-page.component').then((m) => m.EmailPageComponent),
  confluence: () =>
    import('./pages/confluence-activity/confluence-activity-page.component').then(
      (m) => m.ConfluenceActivityPageComponent,
    ),
  repositories: () =>
    import('./pages/repositories/repositories-page.component').then((m) => m.RepositoriesPageComponent),
  periods: () => import('./pages/periods/periods-page.component').then((m) => m.PeriodsPageComponent),
};

export const routes: Routes = [
  { path: '', pathMatch: 'full', redirectTo: 'overview' },
  ...PANES.map((p) => ({ path: p.id, loadComponent: PAGES[p.id] })),
  // A day of the Days view, and the week holding a day ("YYYY-MM-DD", bound to the page's `day` input).
  { path: 'days/:day', loadComponent: PAGES['days'] },
  { path: 'week/:day', loadComponent: PAGES['week'] },
  {
    path: 'settings',
    loadComponent: () => import('./pages/settings/settings-page.component').then((m) => m.SettingsPageComponent),
  },
  { path: '**', redirectTo: 'overview' },
];
