import type { Meta, StoryObj } from '@storybook/angular-vite';
import { applicationConfig } from '@storybook/angular-vite';
import { provideIcons } from '@ng-icons/core';
import * as lucide from '@ng-icons/lucide';
import { expect, fn } from 'storybook/test';
import { PANES, STEPS } from '../../shell/panes';
import { PageLauncherComponent } from './page-launcher.component';

const groups = STEPS.map((step) => ({ step, panes: PANES.filter((p) => !p.view && p.step === step) })).filter(
  (g) => g.panes.length > 0,
);
const icons = Object.fromEntries(PANES.map((p) => [p.icon, (lucide as Record<string, string>)[p.icon]]));

const meta: Meta<PageLauncherComponent> = {
  title: 'UI/Page launcher',
  component: PageLauncherComponent,
  decorators: [applicationConfig({ providers: [provideIcons(icons)] })],
  args: { groups, pinned: ['chats', 'jira'], current: 'plans', open: fn(), togglePin: fn() },
};
export default meta;
type Story = StoryObj<PageLauncherComponent>;

export const Default: Story = {};

/** A pin toggles the page in the side panel; the tile opens it. */
export const PinsAndOpens: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.click(canvas.getByRole('button', { name: 'Pin Plans to the side panel' }));
    await expect(args.togglePin).toHaveBeenCalledWith('plans');
    await userEvent.click(canvas.getByRole('button', { name: 'Chats' }));
    await expect(args.open).toHaveBeenCalledWith('chats');
  },
};
export const Hungarian: Story = { parameters: { lang: 'hu' } };
