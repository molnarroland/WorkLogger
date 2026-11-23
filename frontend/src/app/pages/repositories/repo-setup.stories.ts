import type { Meta, StoryObj } from '@storybook/angular-vite';
import { RepoSetupComponent } from './repo-setup.component';

const meta: Meta<RepoSetupComponent> = { title: 'Pages/Repositories/Setup', component: RepoSetupComponent };
export default meta;
type Story = StoryObj<RepoSetupComponent>;

export const Default: Story = {};
export const Hungarian: Story = { parameters: { lang: 'hu' } };
