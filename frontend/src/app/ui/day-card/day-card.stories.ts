import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { sampleMonth } from '../../api/testing/fixtures';
import { logsOn, octDays } from '../../api/testing/story-data';
import { DayCardComponent } from './day-card.component';

const day = (d: string, today = '2026-10-30') => ({ day: d, info: octDays.get(d), logs: logsOn(d), today });

const meta: Meta<DayCardComponent> = {
  title: 'UI/Day card',
  component: DayCardComponent,
  args: { ...day('2026-10-05'), daySelect: fn(), dropped: fn() },
  render: (args) => ({
    props: args,
    template: `<wl-day-card style="width: 9rem; height: 7rem" [day]="day" [info]="info" [logs]="logs" [today]="today" [selected]="selected" [dropTarget]="dropTarget" (daySelect)="daySelect($event)" (dropped)="dropped($event)" />`,
  }),
};
export default meta;
type Story = StoryObj<DayCardComponent>;

export const Workday: Story = {};
export const Empty: Story = { args: { ...day('2026-10-29'), logs: [] } };
export const Weekend: Story = { args: day('2026-10-04') };
const emptyNovember = new Map(sampleMonth('2026-11', true).days.map((d) => [d.day, d]));
export const Upcoming: Story = { args: { day: '2026-11-03', info: emptyNovember.get('2026-11-03'), logs: [] } };
export const Today: Story = { args: day('2026-10-30') };
export const Selected: Story = { args: { selected: true } };
export const DropTarget: Story = { args: { dropTarget: true } };
export const OtherMonth: Story = { args: { day: '2026-09-30', info: undefined, logs: [] } };
export const Selects: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.click(canvas.getByRole('button'));
    await expect(args.daySelect).toHaveBeenCalledWith('2026-10-05');
  },
};
