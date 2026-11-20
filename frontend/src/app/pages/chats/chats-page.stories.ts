import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, waitFor } from 'storybook/test';
import { ChatsPageComponent } from './chats-page.component';

const meta: Meta<ChatsPageComponent> = { title: 'Pages/Chats', component: ChatsPageComponent };
export default meta;
type Story = StoryObj<ChatsPageComponent>;

export const October: Story = {};
export const Empty: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const OpensAConversation: Story = {
  play: async ({ canvas, userEvent }) => {
    const [first] = await canvas.findAllByRole('option');
    await userEvent.click(first);
    await expect(first).toHaveAttribute('aria-selected', 'true');
  },
};

/** Opened from an entry or a day's panel: that day only, with the conversation open. */
export const OneConversationFromALink: Story = {
  args: { day: '2026-10-28', focus: '2026-10-28|dm/mira' },
  play: async ({ canvas }) => {
    await expect(await canvas.findByLabelText('Day')).toHaveValue('2026-10-28');
    const mira = await canvas.findByRole('option', { name: /Mira Lindqvist/ });
    await waitFor(() => expect(mira).toHaveAttribute('aria-selected', 'true'));
  },
};

/** The search box narrows the list to the conversations whose name or issue matches. */
export const Searches: Story = {
  play: async ({ canvas, userEvent }) => {
    await canvas.findAllByRole('option');
    await userEvent.type(canvas.getByRole('searchbox', { name: 'Search chats' }), 'mira');
    await waitFor(async () => {
      const options = canvas.getAllByRole('option');
      await expect(options.every((o) => /Mira/.test(o.textContent ?? ''))).toBe(true);
    });
    await userEvent.clear(canvas.getByRole('searchbox'));
    await userEvent.type(canvas.getByRole('searchbox'), 'no such chat');
    await expect(await canvas.findByText('Nothing matches the filter')).toBeVisible();
  },
};
