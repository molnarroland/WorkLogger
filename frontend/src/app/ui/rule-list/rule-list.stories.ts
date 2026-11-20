import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { RuleListComponent } from './rule-list.component';

const meta: Meta<RuleListComponent> = {
  title: 'UI/Rule list',
  component: RuleListComponent,
  args: {
    items: [{ label: 'Sprint planning', issueKey: 'TEAM-7' }],
    addLabel: 'Add',
    labelPlaceholder: 'Title contains',
    issuePlaceholder: 'Issue',
    add: fn(),
    remove: fn(),
  },
};
export default meta;
type Story = StoryObj<RuleListComponent>;

export const MeetingRules: Story = {};
export const Projects: Story = {
  args: {
    items: [
      { label: 'ORB', issueKey: 'ORB-1' },
      { label: 'TEAM', issueKey: null },
    ],
    mono: true,
    quoted: false,
    issueOptional: true,
  },
};
export const ReadOnly: Story = { args: { addLabel: '' } };
export const AddsWithEnter: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.type(canvas.getByLabelText('Title contains'), 'Retro');
    await userEvent.type(canvas.getByLabelText('Issue'), 'int-9{Enter}');
    await expect(args.add).toHaveBeenCalledWith({ label: 'Retro', issueKey: 'int-9' }); // the backend normalises keys
  },
};
