import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn, screen } from 'storybook/test';
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
    await userEvent.click(screen.getAllByRole('button', { name: 'Add' })[0]);
    await expect(args.act).toHaveBeenCalledWith(expect.objectContaining({ id: 'hrmaster' }));
  },
};
export const NothingToDo: Story = { args: { steps: [] } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
