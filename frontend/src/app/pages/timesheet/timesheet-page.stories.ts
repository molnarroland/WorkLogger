import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect } from 'storybook/test';
import { TimesheetPageComponent } from './timesheet-page.component';

const meta: Meta<TimesheetPageComponent> = { title: 'Pages/Timesheet', component: TimesheetPageComponent };
export default meta;
type Story = StoryObj<TimesheetPageComponent>;

export const October: Story = {};
export const Empty: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const OpensADay: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole('button', { name: /Oct 14, 2026/ }));
    await expect(await canvas.findByRole('heading', { name: /Oct 14, 2026/ })).toBeVisible();
  },
};
