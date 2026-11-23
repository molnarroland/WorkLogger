import type { Meta, StoryObj } from '@storybook/angular-vite';
import { fn } from 'storybook/test';
import { TaskListComponent } from './task-list.component';

const meta: Meta<TaskListComponent> = {
  title: 'UI/Task list',
  component: TaskListComponent,
  args: {
    cleared: fn(),
    tasks: [
      {
        id: 1,
        label: 'Read back Tempo (2026-11)',
        state: 'done',
        message: '',
        started: '2026-11-17T09:12:01Z',
        ended: '2026-11-17T09:12:03Z',
      },
      {
        id: 2,
        label: 'Fetch work sessions from HRmaster',
        state: 'failed',
        message: 'HRmaster: signed in, but the work-time calendar was not recognised',
        started: '2026-11-17T09:13:00Z',
        ended: '2026-11-17T09:13:40Z',
      },
      { id: 3, label: 'Collect range', state: 'cancelled', message: 'Cancelled.', started: '2026-11-17T09:14:00Z' },
      { id: 4, label: 'Fetch Google Chat', state: 'running', message: '', started: '2026-11-17T09:15:00Z' },
    ],
  },
};
export default meta;
type Story = StoryObj<TaskListComponent>;

export const Mixed: Story = {};
export const Empty: Story = { args: { tasks: [] } };
