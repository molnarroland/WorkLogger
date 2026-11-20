import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, waitFor } from 'storybook/test';
import { PlansPageComponent } from './plans-page.component';

const meta: Meta<PlansPageComponent> = { title: 'Pages/Plans', component: PlansPageComponent };
export default meta;
type Story = StoryObj<PlansPageComponent>;

export const October: Story = {
  play: async ({ canvas }) => {
    await expect((await canvas.findAllByText('Planned')).length).toBeGreaterThan(0);
  },
};
export const Empty: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };

/** The search box narrows the plans to the tickets and texts that match. */
export const Searches: Story = {
  play: async ({ canvas, userEvent }) => {
    await canvas.findAllByText('Planned');
    await userEvent.type(canvas.getByRole('searchbox', { name: 'Search plans' }), 'no such plan');
    await waitFor(() => expect(canvas.getByText('Nothing matches the filter')).toBeVisible());
  },
};
