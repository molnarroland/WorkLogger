import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect } from 'storybook/test';
import { MeetingsPageComponent } from './meetings-page.component';

const meta: Meta<MeetingsPageComponent> = { title: 'Pages/Meetings', component: MeetingsPageComponent };
export default meta;
type Story = StoryObj<MeetingsPageComponent>;

export const October: Story = {};
export const Empty: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };

/** Opened from a day's panel: that day only, the meeting marked. */
export const OneDayFromALink: Story = {
  args: { day: '2026-10-27', focus: '2026-10-27-planning' },
  play: async ({ canvas }) => {
    await expect(await canvas.findByText(/^Only Tue, Oct 27/)).toBeVisible();
    await expect(await canvas.findAllByRole('row')).toHaveLength(2); // the standup and the planning
    const row = canvas.getByText('Sprint planning').closest('tr')!;
    await expect(row).toHaveAttribute('aria-current', 'true');
  },
};
