import { moduleMetadata, type Meta, type StoryObj } from '@storybook/angular-vite';
import { expect, screen } from 'storybook/test';
import { FakeApi } from '../../api/fake-api';
import { EntrySheetsComponent } from './entry-sheets.component';

/** The sheets for a git entry of the sample month, opened by buttons as the inspector's outputs would. */
const meta: Meta<EntrySheetsComponent> = {
  title: 'Shell/Entry sheets',
  component: EntrySheetsComponent,
  decorators: [moduleMetadata({ imports: [EntrySheetsComponent] })],
  loaders: [async () => ({ entry: (await new FakeApi().month('')).worklogs.find((w) => w.source === 'git') })],
  render: (_args, { loaded }) => ({
    props: { entry: loaded['entry'] },
    template: `
      <wl-entry-sheets #sheets />
      <button type="button" (click)="sheets.chooseIssue(entry)">Choose a ticket</button>
      <button type="button" (click)="sheets.showCommits(entry)">Show commits</button>
    `,
  }),
};
export default meta;
type Story = StoryObj<EntrySheetsComponent>;

export const ChooseIssue: Story = {
  play: async ({ userEvent }) => {
    await userEvent.click(await screen.findByRole('button', { name: 'Choose a ticket' }));
    await expect(await screen.findByText(/In this period and before/)).toBeVisible();
  },
};

export const Commits: Story = {
  play: async ({ userEvent }) => {
    await userEvent.click(await screen.findByRole('button', { name: 'Show commits' }));
    await expect(await screen.findByText(/Fix rounding of the last entry/)).toBeVisible();
  },
};
