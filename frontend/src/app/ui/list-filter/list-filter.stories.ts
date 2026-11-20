import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import { ListFilterComponent } from './list-filter.component';

const meta: Meta<ListFilterComponent> = {
  title: 'UI/List filter',
  component: ListFilterComponent,
  args: {
    query: '',
    day: null,
    min: '2026-11-01',
    max: '2026-11-30',
    placeholder: 'Search chats',
    queryChange: fn(),
    dayChange: fn(),
  },
};
export default meta;
type Story = StoryObj<ListFilterComponent>;

export const Default: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await expect(canvas.queryByRole('button', { name: /Clear filter/ })).toBeNull();
    await userEvent.type(canvas.getByRole('searchbox', { name: 'Search chats' }), 'nova');
    await expect(args.queryChange).toHaveBeenLastCalledWith('nova');
    await userEvent.click(canvas.getByRole('button', { name: /Clear filter/ }));
    await expect(args.queryChange).toHaveBeenLastCalledWith('');
  },
};

export const Filtered: Story = { args: { query: 'standup', day: '2026-11-12' } };
export const Hungarian: Story = { args: { query: 'standup', day: '2026-11-12' }, parameters: { lang: 'hu' } };
