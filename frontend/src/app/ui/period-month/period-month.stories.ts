import type { Meta, StoryObj } from '@storybook/angular-vite';
import { defaultSettings, sampleMonth } from '../../api/testing/fixtures';
import { oct, octDays } from '../../api/testing/story-data';
import { PeriodMonthComponent } from './period-month.component';

const november = sampleMonth('2026-11');

const meta: Meta<PeriodMonthComponent> = {
  title: 'UI/Period month',
  component: PeriodMonthComponent,
  args: { month: '2026-10', weeks: oct.weeks, days: octDays, periods: defaultSettings().periods },
};
export default meta;
type Story = StoryObj<PeriodMonthComponent>;

/** The sample's "Map provider migration" period, 19–23 October. */
export const OnePeriod: Story = {};
export const NoPeriods: Story = {
  args: { month: '2026-11', weeks: november.weeks, days: new Map(november.days.map((d) => [d.day, d])) },
};
export const Hungarian: Story = { parameters: { lang: 'hu' } };
