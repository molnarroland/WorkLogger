import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn, screen, within } from 'storybook/test';
import { sampleSetup } from '../../api/testing/fixtures';
import { toSetupSteps } from '../../shared/setup-steps';
import { SetupChecklistComponent } from './setup-checklist.component';

const steps = toSetupSteps(sampleSetup('checklist'));

const meta: Meta<SetupChecklistComponent> = {
  title: 'UI/Setup checklist',
  component: SetupChecklistComponent,
  args: { steps, act: fn() },
};
export default meta;
type Story = StoryObj<SetupChecklistComponent>;

export const PartlyDone: Story = {
  play: async ({ args, userEvent }) => {
    await expect(screen.getByRole('status')).toHaveTextContent('2 of 5 done');
    // A source's step opens its Setup tab ("Open"); accounts are added on Settings ("Add").
    const item = (await screen.findByText('Add your HRmaster site')).closest('sn-checklist-item') as HTMLElement;
    await userEvent.click(within(item).getByRole('button', { name: 'Open' }));
    await expect(args.act).toHaveBeenCalledWith(expect.objectContaining({ id: 'hrmaster' }));
  },
};
export const NothingToDo: Story = { args: { steps: [] } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
