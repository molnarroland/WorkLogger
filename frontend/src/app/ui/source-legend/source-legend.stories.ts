import type { Meta, StoryObj } from '@storybook/angular-vite';
import { oct } from '../../api/testing/story-data';
import { toBarParts } from '../../shared/colors';
import { SourceLegendComponent } from './source-legend.component';

const meta: Meta<SourceLegendComponent> = {
  title: 'UI/Source legend',
  component: SourceLegendComponent,
  args: { parts: toBarParts(oct.summary.barParts) },
};
export default meta;
type Story = StoryObj<SourceLegendComponent>;

export const WithTime: Story = {};
export const NamesOnly: Story = { args: { showTime: false } };
export const Grouped: Story = {
  args: {
    title: 'legend.collected',
    showTime: false,
    parts: [
      { color: 'var(--wl-source-coding)', seconds: 0, label: 'source.git', icon: 'phosphorGitBranch' },
      { color: 'var(--wl-source-meeting)', seconds: 0, label: 'source.meeting', icon: 'phosphorUsers' },
    ],
  },
};
export const Hungarian: Story = { parameters: { lang: 'hu' } };
