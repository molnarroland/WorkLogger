import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { sampleChats, sampleMeetings } from '../../api/testing/fixtures';
import { DayEvidenceComponent } from './day-evidence.component';

/** The 28th has meetings and chats, with the tickets the sample's rules give them. */
const DAY = '2026-10-28';
const meetings = sampleMeetings('2026-10').meetings.filter((m) => m.day === DAY);
const chats = sampleChats('2026-10').days.flatMap((d) => (d.day === DAY ? d.chats : []));

const meta: Meta<DayEvidenceComponent> = {
  title: 'UI/Day evidence',
  component: DayEvidenceComponent,
  args: { meetings, chats, canUse: true, use: fn(), panes: ['meetings', 'chats'], open: fn() },
  render: (args) => ({
    props: args,
    template: `<wl-day-evidence style="width: 18rem; padding: 1rem" [meetings]="meetings" [chats]="chats" [canUse]="canUse" (use)="use($event)" [panes]="panes" (open)="open($event)" />`,
  }),
};
export default meta;
type Story = StoryObj<DayEvidenceComponent>;

export const Day: Story = {};
export const ReadOnly: Story = { args: { canUse: false } };
export const Nothing: Story = { args: { meetings: [], chats: [] } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };

export const UseATicket: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.click(await canvas.findByRole('button', { name: 'Use TEAM-12' }));
    await expect(args.use).toHaveBeenCalledWith('TEAM-12');
  },
};

/** Each meeting and chat opens on its page, narrowed to its day. */
export const OpenOnItsPage: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.click(canvas.getByRole('button', { name: 'Show Mira Lindqvist in Chats' }));
    await expect(args.open).toHaveBeenCalledWith({ pane: 'chats', day: DAY, id: `${DAY}|dm/mira` });
    await userEvent.click(canvas.getByRole('button', { name: 'Show Daily standup in Meetings' }));
    await expect(args.open).toHaveBeenCalledWith({ pane: 'meetings', day: DAY, id: `${DAY}-standup` });
  },
};
