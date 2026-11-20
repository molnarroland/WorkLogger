import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { sampleWeek } from '../../api/testing/fixtures';
import { WeekTimelineComponent } from './week-timeline.component';

const week = (day: string) => {
  const w = sampleWeek(day)!;
  return { days: w.days, logs: w.worklogs };
};

const meta: Meta<WeekTimelineComponent> = {
  title: 'UI/Week timeline',
  component: WeekTimelineComponent,
  args: { ...week('2026-10-27'), today: '2026-10-30', open: fn() },
  render: (args) => ({
    props: args,
    template: `<wl-week-timeline style="height: 40rem" [days]="days" [logs]="logs" [today]="today" (open)="open($event)" />`,
  }),
};
export default meta;
type Story = StoryObj<WeekTimelineComponent>;

/** The last week: a missing ticket on Tuesday, Thursday and Friday still empty. */
export const LastWeek: Story = {};
export const InTempo: Story = { args: week('2026-10-12') };
/** Leave on Friday the 9th. */
export const WithLeave: Story = { args: week('2026-10-06') };
/** Starts in September: the week runs across the month start. */
export const MonthStart: Story = { args: week('2026-10-01') };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const Dark: Story = { globals: { scheme: 'dark' } };

export const OpenADay: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.click(canvas.getByRole('button', { name: /^No issue Code review/ }));
    await expect(args.open).toHaveBeenCalledWith('2026-10-27');
  },
};
