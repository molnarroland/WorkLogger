import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect } from 'storybook/test';
import { OverviewPageComponent } from './overview-page.component';

const meta: Meta<OverviewPageComponent> = { title: 'Pages/Overview', component: OverviewPageComponent };
export default meta;
type Story = StoryObj<OverviewPageComponent>;

export const October: Story = {
  play: async ({ canvas }) => {
    await expect(await canvas.findByRole('heading', { name: /What's left/ })).toBeVisible();
    await expect(await canvas.findByRole('button', { name: 'Open the first day' })).toBeVisible();
    await expect(await canvas.findByText('1 entry without a ticket')).toBeVisible();
  },
};
export const Empty: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const Dark: Story = { globals: { scheme: 'dark' } };
/** axe can't see through the macOS materials (backdrop blur), so only its contrast check is off here. */
export const MacOS: Story = {
  globals: { skin: 'mac' },
  parameters: { a11y: { config: { rules: [{ id: 'color-contrast', enabled: false }] } } },
};
