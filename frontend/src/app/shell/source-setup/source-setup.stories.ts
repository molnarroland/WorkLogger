import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect } from 'storybook/test';
import { SourceSetupComponent } from './source-setup.component';

const meta: Meta<SourceSetupComponent> = { title: 'Shell/Source setup', component: SourceSetupComponent };
export default meta;
type Story = StoryObj<SourceSetupComponent>;

/** Repositories: the folders and commit emails. */
export const Repositories: Story = {
  args: { source: 'repositories' },
  play: async ({ canvas }) => {
    await expect(await canvas.findByText('Your commit emails')).toBeVisible();
  },
};
/** Meetings: the calendars. */
export const Meetings: Story = { args: { source: 'meetings' } };
/** Work sessions: the HRmaster profile's form. */
export const WorkSessions: Story = { args: { source: 'sessions' } };
export const Hungarian: Story = { args: { source: 'repositories' }, parameters: { lang: 'hu' } };
