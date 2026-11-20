import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, screen } from 'storybook/test';
import { JIRA_TOKEN_STEPS, TEMPO_TOKEN_STEPS, TokenHelpComponent } from './token-help.component';

const meta: Meta<TokenHelpComponent> = {
  title: 'UI/Token help',
  component: TokenHelpComponent,
  args: { title: 'jira.tokenTitle', steps: JIRA_TOKEN_STEPS },
};
export default meta;
type Story = StoryObj<TokenHelpComponent>;

export const Jira: Story = {
  play: async () => {
    await expect(await screen.findAllByRole('listitem')).toHaveLength(4);
    await expect(screen.getByText(/read:jira-user and read:jira-work/)).toBeVisible();
  },
};
export const Tempo: Story = { args: { title: 'tempo.tokenTitle', steps: TEMPO_TOKEN_STEPS } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
