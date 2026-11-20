import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, waitFor } from 'storybook/test';
import { EmailPageComponent } from './email-page.component';

const meta: Meta<EmailPageComponent> = { title: 'Pages/Email', component: EmailPageComponent };
export default meta;
type Story = StoryObj<EmailPageComponent>;

/** Threads by day, each mail of a thread under it. */
export const October: Story = {
  play: async ({ canvas }) => {
    await expect((await canvas.findAllByText('RE: Offsite agenda')).length).toBeGreaterThan(0);
  },
};
export const NoEmail: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };

/** The search box narrows the feed to what matches. */
export const Searches: Story = {
  play: async ({ canvas, userEvent }) => {
    await canvas.findAllByText('RE: Offsite agenda');
    await userEvent.type(canvas.getByRole('searchbox', { name: 'Search sent mail' }), 'no such thing');
    await waitFor(() => expect(canvas.getByText('Nothing matches the filter')).toBeVisible());
  },
};
