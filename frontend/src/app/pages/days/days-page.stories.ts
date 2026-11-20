import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, screen, waitFor } from 'storybook/test';
import { DaysPageComponent } from './days-page.component';

const meta: Meta<DaysPageComponent> = { title: 'Pages/Days', component: DaysPageComponent };
export default meta;
type Story = StoryObj<DaysPageComponent>;

/** No day in the route: opens the first one left to do, the 27th with its entry without a ticket. */
export const FirstToDo: Story = {
  play: async ({ canvas }) => {
    await expect(await canvas.findByRole('heading', { name: /October 27/ })).toBeVisible();
    await expect(await canvas.findByRole('textbox', { name: 'Ticket for Code review' })).toBeVisible();
  },
};
export const EmptyDay: Story = { args: { day: '2026-10-29' } };
export const InTempo: Story = { args: { day: '2026-10-14' } };
export const Weekend: Story = { args: { day: '2026-10-31' } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const Dark: Story = { globals: { scheme: 'dark' } };

export const TicketFromAChat: Story = {
  args: { day: '2026-10-27' },
  play: async ({ canvas, userEvent }) => {
    // The standup's ticket (FakeApi's default meeting issue) goes to the entry without one.
    await userEvent.click(await canvas.findByRole('button', { name: 'Use TEAM-12' }, { timeout: 3000 }));
    await waitFor(() => expect(canvas.queryByRole('textbox', { name: 'Ticket for Code review' })).toBeNull());
  },
};

/** A day with drafts can't be sent; reviewed, it is sent by hand after the in-page question (no native alert here). */
export const SendDayToTempo: Story = {
  args: { day: '2026-10-16' },
  play: async ({ canvas, userEvent }) => {
    const send = await canvas.findByRole('button', { name: 'Send day to Tempo' });
    await expect(send).toBeDisabled();
    await userEvent.click(canvas.getByRole('button', { name: 'Use the descriptions' }));
    await waitFor(() => expect(canvas.getByRole('button', { name: 'Mark reviewed and go to next' })).toBeEnabled());
    await userEvent.click(canvas.getByRole('button', { name: 'Mark reviewed and go to next' }));
    await waitFor(() => expect(send).toBeEnabled());
    await userEvent.click(send);
    const alert = await screen.findByRole('alertdialog', {
      name: /^Send 3 worklogs \(.+\) for .*October 16.* to Tempo\?$/,
    });
    await expect(alert).toBeVisible();
    await userEvent.click(screen.getByRole('button', { name: 'Send to Tempo' }));
    await waitFor(() => expect(send).toBeDisabled());
  },
};
