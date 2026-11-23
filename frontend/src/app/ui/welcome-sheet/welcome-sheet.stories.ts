import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn, screen, within } from 'storybook/test';
import { sampleSetup } from '../../api/testing/fixtures';
import { toSetupSteps } from '../../shared/setup-steps';
import { WelcomeSheetComponent } from './welcome-sheet.component';

const steps = toSetupSteps(sampleSetup('welcome'));

const meta: Meta<WelcomeSheetComponent> = {
  title: 'UI/Welcome sheet',
  component: WelcomeSheetComponent,
  args: {
    open: true,
    modules: ['git', 'calendar'],
    tempoByHand: true,
    steps: [],
    modulesChange: fn(),
    tempoByHandChange: fn(),
    openChange: fn(),
    act: fn(),
  },
};
export default meta;
type Story = StoryObj<WelcomeSheetComponent>;

export const FirstRun: Story = {};
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const Accounts: Story = {
  args: { startPage: 'setup', steps },
  play: async ({ args, userEvent }) => {
    // A source's step opens its Setup tab ("Open"); accounts are added on Settings ("Add").
    const item = (await screen.findByText('Add your HRmaster site')).closest('sn-checklist-item') as HTMLElement;
    await userEvent.click(within(item).getByRole('button', { name: 'Open' }));
    await expect(args.act).toHaveBeenCalledWith(expect.objectContaining({ id: 'hrmaster' }));
  },
};
export const ToggleModule: Story = {
  args: { startPage: 'sources' },
  play: async ({ args, userEvent }) => {
    await userEvent.click(await screen.findByRole('switch', { name: 'Chats' }));
    await expect(args.modulesChange).toHaveBeenCalledWith(['git', 'calendar', 'chats']);
  },
};
