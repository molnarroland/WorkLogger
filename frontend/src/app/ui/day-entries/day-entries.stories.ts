import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { logsOn } from '../../api/testing/story-data';
import { DayEntriesComponent } from './day-entries.component';

const meta: Meta<DayEntriesComponent> = {
  title: 'UI/Day entries',
  component: DayEntriesComponent,
  args: { selected: null, issue: fn(), comment: fn(), selectedChange: fn() },
  render: (args) => ({
    props: args,
    template: `<wl-day-entries style="padding: 1rem; max-width: 56rem" [logs]="logs" [(selected)]="selected" (issue)="issue($event)" (comment)="comment($event)" (selectedChange)="selectedChange($event)" />`,
  }),
};
export default meta;
type Story = StoryObj<DayEntriesComponent>;

/** The 27th: a manual entry without a ticket among drafts. */
export const NeedsTicket: Story = { args: { logs: logsOn('2026-10-27') } };
export const InTempo: Story = { args: { logs: logsOn('2026-10-14') } };
export const Selected: Story = { args: { logs: logsOn('2026-10-27'), selected: logsOn('2026-10-27')[1].id } };
export const Hungarian: Story = { args: { logs: logsOn('2026-10-27') }, parameters: { lang: 'hu' } };

export const TypeATicket: Story = {
  args: { logs: logsOn('2026-10-27') },
  play: async ({ canvas, userEvent, args }) => {
    const field = canvas.getByRole('textbox', { name: 'Ticket for Code review' });
    await userEvent.type(field, 'proj-123{Enter}');
    const id = logsOn('2026-10-27').find((w) => !w.issueKey)!.id;
    await expect(args.issue).toHaveBeenCalledWith({ id, key: 'proj-123' }); // the backend normalises keys
  },
};

export const SelectAnEntry: Story = {
  args: { logs: logsOn('2026-10-27') },
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.click(canvas.getByRole('button', { name: 'Daily standup' }));
    await expect(args.selectedChange).toHaveBeenCalledWith(logsOn('2026-10-27')[0].id);
  },
};
