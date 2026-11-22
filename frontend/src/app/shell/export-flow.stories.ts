import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn, screen } from 'storybook/test';
import { ExportFlowComponent } from './export-flow.component';

const meta: Meta<ExportFlowComponent> = {
  title: 'Shell/Export flow',
  component: ExportFlowComponent,
  args: { open: true, openChange: fn() },
};
export default meta;
type Story = StoryObj<ExportFlowComponent>;

export const Open: Story = {};

/** The period's reviewed entries can go to Tempo at once: the sheet closes, then the store asks first. */
export const SendsThePeriod: Story = {
  play: async ({ args, userEvent }) => {
    await expect(await screen.findByText(/Days that still have drafts are left out/)).toBeVisible();
    await userEvent.click(await screen.findByRole('button', { name: /Send reviewed entries/ }));
    await expect(args.openChange).toHaveBeenCalledWith(false);
  },
};
