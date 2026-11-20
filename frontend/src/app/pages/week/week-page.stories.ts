import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect } from 'storybook/test';
import { WeekPageComponent } from './week-page.component';

const meta: Meta<WeekPageComponent> = { title: 'Pages/Week', component: WeekPageComponent };
export default meta;
type Story = StoryObj<WeekPageComponent>;

/** Today's week (the stories' today is Fri 30 Oct). */
export const ThisWeek: Story = {
  play: async ({ canvas }) => {
    await expect(await canvas.findByRole('button', { name: /^No issue Code review/ })).toBeVisible();
  },
};
export const MidMonth: Story = { args: { day: '2026-10-14' } };
export const Empty: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
