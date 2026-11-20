import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { ChipListComponent } from './chip-list.component';

const meta: Meta<ChipListComponent> = {
  title: 'UI/Chip list',
  component: ChipListComponent,
  args: { values: ['Lunch', 'Focus time'], label: 'Add keyword', placeholder: 'e.g. Lunch', valuesChange: fn() },
};
export default meta;
type Story = StoryObj<ChipListComponent>;

export const Keywords: Story = {};
export const Highlighted: Story = { args: { values: ['me@company.com'], highlighted: true } };
export const Empty: Story = { args: { values: [] } };
export const AddsSeveral: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.type(canvas.getByLabelText('Add keyword'), 'Standup, 1:1{Enter}');
    await expect(args.valuesChange).toHaveBeenCalledWith(['Lunch', 'Focus time', 'Standup', '1:1']);
  },
};
