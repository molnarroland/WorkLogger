import type { Meta, StoryObj } from '@storybook/angular-vite';
import { CalendarSettingsComponent } from './calendar-settings.component';

const meta: Meta<CalendarSettingsComponent> = {
  title: 'Pages/Settings/Calendars',
  component: CalendarSettingsComponent,
};
export default meta;
type Story = StoryObj<CalendarSettingsComponent>;

export const Default: Story = {};
export const Hungarian: Story = { parameters: { lang: 'hu' } };
