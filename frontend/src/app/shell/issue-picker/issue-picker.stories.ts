import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn, screen } from 'storybook/test';
import { IssuePickerComponent } from './issue-picker.component';

/** A day of the sample month: the fake offers the month's tickets and invents Jira issues around the day. */
const meta: Meta<IssuePickerComponent> = {
  title: 'Shell/Issue picker',
  component: IssuePickerComponent,
  args: { open: true, day: '2026-09-02', subject: 'Fix rounding of the last entry', picked: fn(), openChange: fn() },
};
export default meta;
type Story = StoryObj<IssuePickerComponent>;

export const Known: Story = {
  play: async () => {
    await expect(await screen.findByText('ORB-231')).toBeVisible();
  },
};

/** Typing filters the known tickets; Search Jira finds issues created around the day, with their descriptions. */
export const SearchJira: Story = {
  play: async ({ args, userEvent }) => {
    await userEvent.type(await screen.findByRole('searchbox'), 'export');
    await userEvent.click(screen.getByRole('button', { name: /Search Jira/ }));
    const hit = await screen.findByText('Export the monthly report as CSV');
    await expect(screen.getByText(/^Finance wants the month as a CSV/)).toBeVisible();
    await userEvent.click(hit);
    await expect(args.picked).toHaveBeenCalledWith('WL-121');
  },
};

/** A key typed in full is offered as it is. */
export const TypedKey: Story = {
  play: async ({ args, userEvent }) => {
    await userEvent.type(await screen.findByRole('searchbox'), 'abc-42');
    await userEvent.click(await screen.findByRole('button', { name: 'Use ABC-42' }));
    await expect(args.picked).toHaveBeenCalledWith('ABC-42');
  },
};

export const Hungarian: Story = { parameters: { lang: 'hu' } };
