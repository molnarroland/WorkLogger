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

/** The welcome page: the month's three steps. */
export const Welcome: Story = {
  play: async () => {
    await expect(await screen.findByText('Check each day')).toBeVisible();
  },
};

/** A "yes" to HRmaster opens its profile form; then the first Collect, which ends on the first day to do. */
export const SetUpHrmasterThenCollect: Story = {
  play: async ({ userEvent }) => {
    await userEvent.click(await screen.findByRole('button', { name: 'Start setup' }));
    const next = await screen.findByRole('button', { name: 'Next' });
    await expect(next).toBeDisabled();
    await userEvent.click(await screen.findByRole('switch', { name: 'Do you use HRmaster?' }));
    await userEvent.click(next);
    await expect(await screen.findByText('Add your HRmaster site and sign in.')).toBeVisible();
    await expect(screen.getByRole('button', { name: 'Skip for now' })).toBeVisible();
    await userEvent.click(screen.getByRole('button', { name: 'Next' }));
    await expect(await screen.findByRole('heading', { name: /^Collecting / })).toBeVisible();
    await waitFor(
      () => expect(screen.getByRole('button', { name: /^(Go to the first day|Open the overview)$/ })).toBeEnabled(),
      {
        timeout: 5000,
      },
    );
  },
};
