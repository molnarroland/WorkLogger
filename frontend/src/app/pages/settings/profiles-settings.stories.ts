import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, screen } from 'storybook/test';
import { freshSettings } from '../../api/testing/fixtures';
import { ProfilesSettingsComponent } from './profiles-settings.component';

const meta: Meta<ProfilesSettingsComponent> = {
  title: 'Pages/Settings/Profiles',
  component: ProfilesSettingsComponent,
};
export default meta;
type Story = StoryObj<ProfilesSettingsComponent>;

export const AllKinds: Story = {};
export const Empty: Story = { parameters: { api: { settings: { ...freshSettings(), welcomed: true } } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const AddsAnHRmasterProfile: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole('button', { name: /Add profile/ }));
    await userEvent.click(await screen.findByRole('menuitem', { name: /HRmaster/ }));
    await expect(await canvas.findByLabelText('HRmaster address')).toHaveValue('');
  },
};
