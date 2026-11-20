import type { Meta, StoryObj } from '@storybook/angular-vite';
import { oct } from '../../api/testing/story-data';
import { SOURCE_COLOR } from '../../shared/colors';
import { StackedBarComponent } from './stacked-bar.component';

const columns = oct.reports.week.map((g) => ({
  label: g.id,
  parts: g.bySource.map((s) => ({ color: SOURCE_COLOR[s.source], seconds: s.seconds, label: `source.${s.source}` })),
}));

const meta: Meta<StackedBarComponent> = { title: 'UI/Stacked bar', component: StackedBarComponent, args: { columns } };
export default meta;
type Story = StoryObj<StackedBarComponent>;

export const Weeks: Story = {};
export const Empty: Story = { args: { columns: [] } };
