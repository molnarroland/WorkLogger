import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, waitFor } from 'storybook/test';
import { SessionsPageComponent } from './sessions-page.component';

const meta: Meta<SessionsPageComponent> = { title: 'Pages/Work sessions', component: SessionsPageComponent };
export default meta;
type Story = StoryObj<SessionsPageComponent>;

export const October: Story = {};
export const Empty: Story = { parameters: { api: { empty: true } } };
export const SignedOut: Story = { parameters: { api: { signedIn: { hrmaster: false } } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };

/** Opened from an absence entry: its day is marked. */
export const OneDayFromALink: Story = {
  args: { day: '2026-10-09' },
  play: async ({ canvasElement }) => {
    await waitFor(() => expect(canvasElement.querySelector('[aria-current=true]')?.textContent).toMatch(/Oct 9/));
  },
};
