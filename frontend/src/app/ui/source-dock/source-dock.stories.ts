import type { Meta, StoryObj } from '@storybook/angular-vite';
import { SourceDockComponent } from './source-dock.component';

const meta: Meta<SourceDockComponent> = {
  title: 'UI/Source dock',
  component: SourceDockComponent,
  args: {
    active: 'meetings',
    items: [
      { id: 'repositories', label: 'pane.repositories', icon: 'lucideHardDrive', count: 2 },
      { id: 'meetings', label: 'pane.meetings', icon: 'lucideUsers', count: 4 },
      { id: 'sessions', label: 'pane.sessions', icon: 'lucideClock', count: 0 },
      { id: 'chats', label: 'pane.chats', icon: 'lucideMessagesSquare', count: 12 },
      { id: 'jira', label: 'pane.jira', icon: 'lucideActivity', count: 120 },
      { id: 'periods', label: 'pane.periods', icon: 'lucideFlag', count: 1 },
      { id: 'timesheet', label: 'pane.timesheet', icon: 'lucideCalendar', count: 3, end: true },
      { id: 'review', label: 'pane.review', icon: 'lucideListChecks', count: 7, end: true },
    ],
  },
};
export default meta;
type Story = StoryObj<SourceDockComponent>;

export const Day: Story = {};
export const Empty: Story = { args: { active: '', items: [] } };
