import type { Meta, StoryObj } from '@storybook/angular-vite';
import { fn } from 'storybook/test';
import { oct, octDays } from '../../api/testing/story-data';
import { MonthGridComponent } from './month-grid.component';

const meta: Meta<MonthGridComponent> = {
  title: 'UI/Month grid',
  component: MonthGridComponent,
  args: {
    weeks: oct.weeks,
    days: octDays,
    worklogs: oct.worklogs,
    today: '2026-10-30',
    selected: null,
    daySelect: fn(),
    dropped: fn(),
  },
};
export default meta;
type Story = StoryObj<MonthGridComponent>;

export const October: Story = {};
export const DaySelected: Story = { args: { selected: '2026-10-14' } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
