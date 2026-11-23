import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect } from 'storybook/test';
import { SettingsPageComponent } from './settings-page.component';

const meta: Meta<SettingsPageComponent> = { title: 'Pages/Settings', component: SettingsPageComponent };
export default meta;
type Story = StoryObj<SettingsPageComponent>;

export const General: Story = {};
const onTab = (tab: string) => ({ template: `<wl-settings-page tab="${tab}" />` });
export const Timesheet: Story = { render: () => onTab('timesheet') };
export const Profiles: Story = { render: () => onTab('profiles') };
export const FromWelcomeToJira: Story = { render: () => onTab('jira') };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const SwitchesTab: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole('tab', { name: 'Coding' }));
    await expect(await canvas.findByText('Commit estimates')).toBeVisible();
  },
};
