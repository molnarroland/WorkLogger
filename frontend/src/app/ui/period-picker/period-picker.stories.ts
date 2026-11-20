import type { Meta, StoryObj } from '@storybook/angular-vite';
import { PeriodPickerComponent } from './period-picker.component';

const meta: Meta<PeriodPickerComponent> = {
  title: 'UI/Period picker',
  component: PeriodPickerComponent,
  args: { mode: 'day', value: '2026-10-21', today: '2026-10-30', lang: 'en' },
};
export default meta;
type Story = StoryObj<PeriodPickerComponent>;

export const Day: Story = {};
export const Week: Story = { args: { mode: 'week' } };
export const Month: Story = { args: { mode: 'month', value: '2026-10' } };
