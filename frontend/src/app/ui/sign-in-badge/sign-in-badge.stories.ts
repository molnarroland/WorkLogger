import type { Meta, StoryObj } from '@storybook/angular-vite';
import { SignInBadgeComponent } from './sign-in-badge.component';

const meta: Meta<SignInBadgeComponent> = {
  title: 'UI/Sign-in badge',
  component: SignInBadgeComponent,
  args: { state: true },
};
export default meta;
type Story = StoryObj<SignInBadgeComponent>;

export const SignedIn: Story = {};
export const SignedOut: Story = { args: { state: false } };
export const NotChecked: Story = { args: { state: null } };
