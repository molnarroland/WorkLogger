import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, waitFor } from 'storybook/test';
import { ConfluenceActivityPageComponent } from './confluence-activity-page.component';

const meta: Meta<ConfluenceActivityPageComponent> = {
  title: 'Pages/Confluence activity',
  component: ConfluenceActivityPageComponent,
};
export default meta;
type Story = StoryObj<ConfluenceActivityPageComponent>;

export const October: Story = {
  play: async ({ canvas }) => {
    await expect((await canvas.findAllByRole('button', { name: /Roadmap/ })).length).toBeGreaterThan(0);
  },
};
/** Comments show their text; a page's changes are read when asked for. */
export const ShowsChanges: Story = {
  play: async ({ canvas, userEvent }) => {
    await expect((await canvas.findAllByText('Moved the export to Q3, see the spec.')).length).toBeGreaterThan(0);
    await userEvent.click((await canvas.findAllByRole('button', { name: 'Show changes' }))[0]);
    await expect(await canvas.findByText('− Exports in Q2')).toBeVisible();
    await expect(canvas.getByText('Agreed with the team', { exact: false })).toBeVisible();
  },
};
export const NoConfluence: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };

/** The search box narrows the feed to what matches. */
export const Searches: Story = {
  play: async ({ canvas, userEvent }) => {
    await canvas.findAllByRole('button', { name: /Roadmap/ });
    await userEvent.type(canvas.getByRole('searchbox', { name: 'Search Confluence activity' }), 'no such thing');
    await waitFor(() => expect(canvas.getByText('Nothing matches the filter')).toBeVisible());
  },
};
