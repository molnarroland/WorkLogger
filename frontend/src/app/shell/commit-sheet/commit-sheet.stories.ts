import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn, screen } from 'storybook/test';
import { CommitSheetComponent } from './commit-sheet.component';

/** A git entry of the sample month (the fake makes up two commits for it). */
const meta: Meta<CommitSheetComponent> = {
  title: 'Shell/Commit sheet',
  component: CommitSheetComponent,
  args: { open: true, worklogId: 68, title: 'ORB-227', openChange: fn() },
};
export default meta;
type Story = StoryObj<CommitSheetComponent>;

export const Commits: Story = {
  play: async () => {
    await expect(await screen.findByText(/Fix rounding of the last entry/)).toBeVisible();
  },
};

/** A commit opened, then one file's changes. */
export const FileDiff: Story = {
  play: async ({ userEvent }) => {
    await userEvent.click(await screen.findByRole('button', { name: /Fix rounding of the last entry/ }));
    await userEvent.click(await screen.findByRole('button', { name: /internal\/core\/builder\.go/ }));
    await expect(await screen.findByText(/out\[largest\(parts\)\] \+= rest/)).toBeVisible();
  },
};

/** An entry that wasn't made from commits. */
export const NoCommits: Story = { args: { worklogId: -1 } };

export const Hungarian: Story = { parameters: { lang: 'hu' } };
