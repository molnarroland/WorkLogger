import type { Meta, StoryObj } from '@storybook/angular-vite';
import type { SessionDto } from '../../api/api';
import { at } from '../../shared/worklogs';
import { DayStripComponent } from './day-strip.component';

const minute = (hm: string) => Number(hm.slice(0, 2)) * 60 + Number(hm.slice(3));
const s = (start: string, end: string, kind: 'work' | 'pause' = 'work'): SessionDto => ({
  id: 0,
  day: '2026-10-05',
  start: at('2026-10-05', start),
  end: at('2026-10-05', end),
  kind,
  label: '',
  startMinute: minute(start),
  endMinute: minute(end),
});

const meta: Meta<DayStripComponent> = {
  title: 'UI/Day strip',
  component: DayStripComponent,
  args: { sessions: [s('08:30', '17:10'), s('12:00', '12:30', 'pause')] },
};
export default meta;
type Story = StoryObj<DayStripComponent>;

export const Workday: Story = {};
export const LateNight: Story = { args: { sessions: [s('09:00', '12:00'), s('20:00', '23:30')] } };
export const NoSessions: Story = { args: { sessions: [] } };
