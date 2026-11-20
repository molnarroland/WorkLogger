import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { DayFilterComponent } from './day-filter.component';

const meta: Meta<DayFilterComponent> = {
  title: 'UI/Day filter',
  component: DayFilterComponent,
  args: { day: '2026-10-27', cleared: fn() },
};
export default meta;
type Story = StoryObj<DayFilterComponent>;

export const Default: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await expect(canvas.getByText(/Tue, Oct 27/)).toBeVisible();
    await userEvent.click(canvas.getByRole('button', { name: 'Show all' }));
    await expect(args.cleared).toHaveBeenCalled();
  },
};
export const Hungarian: Story = { parameters: { lang: 'hu' } };
