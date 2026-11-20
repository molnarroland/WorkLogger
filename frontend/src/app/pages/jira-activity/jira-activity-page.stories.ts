import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, waitFor } from 'storybook/test';
import { JiraActivityPageComponent } from './jira-activity-page.component';

const meta: Meta<JiraActivityPageComponent> = { title: 'Pages/Jira activity', component: JiraActivityPageComponent };
export default meta;
type Story = StoryObj<JiraActivityPageComponent>;

export const October: Story = {
  play: async ({ canvas }) => {
    await expect((await canvas.findAllByText('Pushed the fix, ready for review.')).length).toBeGreaterThan(0);
  },
};
/** Entries without a ticket that ran around a ticket's activity are offered its key. */
export const Suggests: Story = {
  play: async ({ canvas }) => {
    await expect((await canvas.findAllByRole('button', { name: /^Use / })).length).toBeGreaterThan(0);
  },
};
export const NoJira: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };

/** The search box narrows the feed to what matches. */
export const Searches: Story = {
  play: async ({ canvas, userEvent }) => {
    await canvas.findAllByText('Pushed the fix, ready for review.');
    await userEvent.type(canvas.getByRole('searchbox', { name: 'Search Jira activity' }), 'no such thing');
    await waitFor(() => expect(canvas.getByText('Nothing matches the filter')).toBeVisible());
  },
};
