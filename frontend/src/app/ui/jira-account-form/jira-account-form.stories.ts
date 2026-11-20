import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { defaultSettings } from '../../api/testing/fixtures';
import { JiraAccountFormComponent } from './jira-account-form.component';

const account = defaultSettings().jiraAccounts[0];

const meta: Meta<JiraAccountFormComponent> = {
  title: 'UI/Jira account form',
  component: JiraAccountFormComponent,
  args: {
    account,
    hasJiraToken: true,
    tempo: 'Tempo',
    checkLines: [],
    checking: false,
    loadingProjects: false,
    changed: fn(),
    token: fn(),
    readProjects: fn(),
    check: fn(),
  },
};
export default meta;
type Story = StoryObj<JiraAccountFormComponent>;

export const LinkedToTempo: Story = {};
export const JiraOnly: Story = { args: { tempo: null, account: { ...account, provider: 'jira' } } };
export const CheckFailed: Story = {
  args: {
    checkLines: [
      { title: 'Signed in to Jira', detail: 'as Me', ok: true },
      { title: 'Tempo worklogs', detail: 'The token lacks “View worklogs”.', ok: false },
      { title: 'Send to Tempo', detail: '', ok: null },
    ],
  },
};
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const ChecksJira: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.click(canvas.getByRole('button', { name: 'Check Jira' }));
    await expect(args.check).toHaveBeenCalledWith('jira');
  },
};
