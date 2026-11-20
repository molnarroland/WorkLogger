import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect } from 'storybook/test';
import { ByTicketPageComponent } from './by-ticket-page.component';

const meta: Meta<ByTicketPageComponent> = { title: 'Pages/By ticket', component: ByTicketPageComponent };
export default meta;
type Story = StoryObj<ByTicketPageComponent>;

export const October: Story = {};
export const Empty: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const PicksATicket: Story = {
  play: async ({ canvas, userEvent }) => {
    const options = await canvas.findAllByRole('option');
    await userEvent.click(options[options.length - 1]);
    await expect(options[options.length - 1]).toHaveAttribute('aria-selected', 'true');
  },
};
