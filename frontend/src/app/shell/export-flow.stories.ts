import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, screen } from 'storybook/test';
import { ExportFlowComponent } from './export-flow.component';

const meta: Meta<ExportFlowComponent> = {
  title: 'Shell/Export flow',
  component: ExportFlowComponent,
  args: { open: true },
};
export default meta;
type Story = StoryObj<ExportFlowComponent>;

export const Open: Story = {};

/** Sending to Tempo isn't offered for the whole range: the sheet points to the Days page instead. */
export const SendsDayByDay: Story = {
  play: async () => {
    await expect(await screen.findByText(/one day at a time/)).toBeVisible();
    await expect(screen.queryByRole('button', { name: /Push to Tempo/ })).toBeNull();
  },
};
