import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect } from 'storybook/test';
import { ReviewPageComponent } from './review-page.component';

const meta: Meta<ReviewPageComponent> = { title: 'Pages/Review', component: ReviewPageComponent };
export default meta;
type Story = StoryObj<ReviewPageComponent>;

export const October: Story = {};
export const Empty: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const SelectOpensInspector: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole('button', { name: 'All' }));
    const rows = await canvas.findAllByRole('row');
    await userEvent.click(rows.find((r) => r.hasAttribute('aria-selected'))!);
    await expect(await canvas.findByLabelText('Duration')).toBeVisible();
  },
};
