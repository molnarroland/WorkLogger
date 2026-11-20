import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { defaultSettings } from '../../api/testing/fixtures';
import { ProfileEditorComponent } from './profile-editor.component';

const [google, hrmaster, tempo] = defaultSettings().profiles;
const confluence = { ...tempo, id: 'c1', kind: 'confluence' as const, url: '', jiraAccountId: 'j1' };

const meta: Meta<ProfileEditorComponent> = {
  title: 'UI/Profile editor',
  component: ProfileEditorComponent,
  args: {
    profile: google,
    signedIn: true,
    jiraAccounts: [{ id: 'j1', title: 'Example Travel' }],
    hasToken: false,
    checkLines: [],
    checking: false,
    changed: fn(),
    signIn: fn(),
    signOut: fn(),
    token: fn(),
    check: fn(),
  },
};
export default meta;
type Story = StoryObj<ProfileEditorComponent>;

export const Google: Story = {};
export const HRmasterSignedOut: Story = { args: { profile: hrmaster, signedIn: false } };
export const Tempo: Story = { args: { profile: tempo } };
export const TempoCheckFailed: Story = {
  args: {
    profile: tempo,
    checkLines: [{ title: 'Tempo worklogs', detail: 'The token lacks “View worklogs”.', ok: false }],
  },
};
export const ConfluenceLinkedToJira: Story = { args: { profile: confluence } };
export const ConfluenceWithOwnSite: Story = {
  args: { profile: { ...confluence, jiraAccountId: '', url: 'https://wiki.example.com' }, hasToken: true },
};
export const Email: Story = {
  args: {
    profile: { ...tempo, id: 'm1', kind: 'email', url: 'imap.gmail.com', email: 'me@example.com', jiraAccountId: '' },
  },
};
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const EditsTheAddress: Story = {
  args: { profile: hrmaster },
  play: async ({ canvas, userEvent, args }) => {
    const field = canvas.getByLabelText('HRmaster address');
    await userEvent.clear(field);
    await userEvent.type(field, 'https://other.hrmaster.hu');
    await userEvent.tab();
    await expect(args.changed).toHaveBeenCalledWith(expect.objectContaining({ url: 'https://other.hrmaster.hu' }));
  },
};
