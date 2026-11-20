import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { ActivityConsoleComponent } from './activity-console.component';

const meta: Meta<ActivityConsoleComponent> = {
  title: 'UI/Activity console',
  component: ActivityConsoleComponent,
  args: {
    closed: fn(),
    lines: [
      {
        time: '2026-10-30T09:12:01Z',
        source: 'git',
        level: 'info',
        message: 'git log in ~/Workspaces/proj (18 commits)',
        replace: false,
      },
      {
        time: '2026-10-30T09:12:03Z',
        source: 'tempo',
        level: 'progress',
        message: 'GET https://api.tempo.io/4/worklogs?… 200',
        replace: false,
      },
      {
        time: '2026-10-30T09:12:04Z',
        source: 'hrmaster',
        level: 'error',
        message: 'Signed out: sign in to HRmaster',
        replace: false,
      },
    ],
  },
  render: (args) => ({
    props: args,
    template: `<wl-activity-console style="height: 240px" [lines]="lines" (closed)="closed()" />`,
  }),
};
export default meta;
type Story = StoryObj<ActivityConsoleComponent>;

export const WithLines: Story = {};
export const Empty: Story = { args: { lines: [] } };
export const Closes: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.click(canvas.getByRole('button', { name: 'Close' }));
    await expect(args.closed).toHaveBeenCalled();
  },
};
