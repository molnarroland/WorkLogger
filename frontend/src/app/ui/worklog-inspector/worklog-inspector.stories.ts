import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { fixtures } from '../../api/testing/fixtures';
import { oct } from '../../api/testing/story-data';
import { WorklogInspectorComponent } from './worklog-inspector.component';

const git = oct.worklogs.find((w) => w.source === 'git' && w.issueKey && w.status === 'draft')!;
const noTicket = oct.worklogs.find((w) => w.issueKey === null) ?? { ...git, issueKey: null };

const meta: Meta<WorklogInspectorComponent> = {
  title: 'UI/Worklog inspector',
  component: WorklogInspectorComponent,
  args: {
    worklog: git,
    changed: fn(),
    duration: fn(),
    remove: fn(),
    panes: ['repositories', 'meetings', 'chats'],
    open: fn(),
  },
  render: (args) => ({
    props: args,
    template: `<div style="max-width: 22rem; padding: 1rem"><wl-worklog-inspector [worklog]="worklog" (changed)="changed($event)" (duration)="duration($event)" (remove)="remove($event)" [panes]="panes" (open)="open($event)" /></div>`,
  }),
};
export default meta;
type Story = StoryObj<WorklogInspectorComponent>;

export const Draft: Story = {};
export const NeedsTicket: Story = { args: { worklog: noTicket } };
// The backend's flags follow the comment: with one, the draft may be marked reviewed.
export const WithComment: Story = {
  args: {
    worklog: { ...git, comment: 'Offline mode fix', tempoText: 'Offline mode fix', needsComment: false },
  },
};
export const SentToTempo: Story = { args: { worklog: fixtures.exportedEntry } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const EditDuration: Story = {
  play: async ({ canvas, userEvent, args }) => {
    const field = canvas.getByLabelText('Duration');
    await userEvent.clear(field);
    await userEvent.type(field, '1h 30m{Enter}');
    await expect(args.duration).toHaveBeenCalledWith({ id: git.id, text: '1h 30m' });
  },
};
export const WritesTheComment: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.type(canvas.getByLabelText('Comment for Tempo'), 'Offline mode fix');
    await userEvent.tab();
    await expect(args.changed).toHaveBeenCalledWith(expect.objectContaining({ comment: 'Offline mode fix' }));
  },
};

/** An entry made from commits opens its repository; the button is there only while that page is switched on. */
export const ShowsWhereItCameFrom: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.click(canvas.getByRole('button', { name: 'Show in Repositories' }));
    await expect(args.open).toHaveBeenCalledWith(git.focus);
  },
};
export const SourcePageSwitchedOff: Story = {
  args: { panes: ['meetings'] },
  play: async ({ canvas }) => {
    await expect(canvas.queryByRole('button', { name: /^Show in/ })).toBeNull();
  },
};
