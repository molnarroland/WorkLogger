import type { Meta, StoryObj } from '@storybook/angular-vite';
import { TotalsPageComponent } from './totals-page.component';

const meta: Meta<TotalsPageComponent> = { title: 'Pages/Totals', component: TotalsPageComponent };
export default meta;
type Story = StoryObj<TotalsPageComponent>;

export const October: Story = {};
export const Empty: Story = { parameters: { api: { empty: true } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
