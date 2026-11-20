import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect } from 'storybook/test';
import { RepositoriesPageComponent } from './repositories-page.component';

const meta: Meta<RepositoriesPageComponent> = { title: 'Pages/Repositories', component: RepositoriesPageComponent };
export default meta;
type Story = StoryObj<RepositoriesPageComponent>;

export const October: Story = {};
export const Empty: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };

/** Opened from an entry made from its commits: the repository is marked. */
export const OneRepositoryFromALink: Story = {
  args: { focus: '2' },
  play: async ({ canvas }) => {
    const row = (await canvas.findByText('web-dashboard')).closest('tr')!;
    await expect(row).toHaveAttribute('aria-current', 'true');
  },
};

/** A day picked: only the repositories with commits that day, with that day's count. */
export const OneDay: Story = {
  args: { day: '2026-10-13' },
  play: async ({ canvas }) => {
    await expect(await canvas.findByText('web-dashboard')).toBeVisible();
    await expect(canvas.queryByText('trip-planner')).toBeNull();
    await expect(canvas.getByText(/4 that day/)).toBeVisible();
  },
};

/** The search box matches the name, the path and the issue and project keys. */
export const Searches: Story = {
  play: async ({ canvas, userEvent }) => {
    await canvas.findByText('trip-planner');
    await userEvent.type(canvas.getByRole('searchbox', { name: 'Search repositories' }), 'nova');
    await expect(canvas.queryByText('trip-planner')).toBeNull();
    await expect(canvas.getByText('web-dashboard')).toBeVisible();
  },
};
