import type { Meta, StoryObj } from '@storybook/angular-vite';
import { freshSettings } from '../../api/testing/fixtures';
import { PeriodsPageComponent } from './periods-page.component';

const meta: Meta<PeriodsPageComponent> = { title: 'Pages/Periods', component: PeriodsPageComponent };
export default meta;
type Story = StoryObj<PeriodsPageComponent>;

export const October: Story = {};
export const Empty: Story = { parameters: { api: { settings: { ...freshSettings(), welcomed: true } } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
