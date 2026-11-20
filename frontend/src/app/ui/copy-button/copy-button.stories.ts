import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect } from 'storybook/test';
import { CopyButtonComponent } from './copy-button.component';

const meta: Meta<CopyButtonComponent> = {
  title: 'UI/Copy button',
  component: CopyButtonComponent,
  args: { label: 'Copy key', text: 'ORB-214' },
};
export default meta;
type Story = StoryObj<CopyButtonComponent>;

export const Standard: Story = {};
export const Prominent: Story = { args: { prominent: true, label: 'Copy all' } };
export const Small: Story = { args: { small: true, label: 'Duration', ariaLabel: 'Copy duration for Mon, Oct 5' } };
export const IconOnly: Story = {
  args: { iconOnly: true, small: true, label: 'Copy text', ariaLabel: 'Copy text for Mon, Oct 5' },
};
export const SaysCopied: Story = {
  play: async ({ canvas, userEvent }) => {
    Object.defineProperty(navigator, 'clipboard', { value: { writeText: async () => undefined }, configurable: true });
    await userEvent.click(canvas.getByRole('button'));
    await expect(await canvas.findByText('Copied')).toBeVisible();
  },
};
