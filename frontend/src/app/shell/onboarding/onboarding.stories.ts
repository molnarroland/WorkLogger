import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, screen, waitFor } from 'storybook/test';
import { freshSettings } from '../../api/testing/fixtures';
import { OnboardingComponent } from './onboarding.component';

const meta: Meta<OnboardingComponent> = {
  title: 'Shell/Onboarding',
  component: OnboardingComponent,
  parameters: { api: { settings: freshSettings() } },
};
export default meta;
type Story = StoryObj<OnboardingComponent>;

export const Questions: Story = {};
export const Hungarian: Story = { parameters: { lang: 'hu' } };

/** A "yes" to HRmaster opens its profile form; Done waits until something is set up. */
export const SetUpHrmaster: Story = {
  play: async ({ userEvent }) => {
    const next = await screen.findByRole('button', { name: 'Next' });
    await expect(next).toBeDisabled();
    await userEvent.click(await screen.findByRole('switch', { name: 'Do you use HRmaster?' }));
    await userEvent.click(next);
    await expect(await screen.findByText('Add your HRmaster site and sign in.')).toBeVisible();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Done' })).toBeEnabled());
  },
};
