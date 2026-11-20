import type { Meta, StoryObj } from '@storybook/angular-vite';
import { octDays } from '../../api/testing/story-data';
import { toBarParts } from '../../shared/colors';
import { SourceBarComponent } from './source-bar.component';

const meta: Meta<SourceBarComponent> = {
  title: 'UI/Source bar',
  component: SourceBarComponent,
  args: { parts: toBarParts(octDays.get('2026-10-05')!.parts), scale: 8 * 3600, height: 0.75 },
  render: (args) => ({
    props: args,
    template: `<div style="padding: 1rem; max-width: 20rem"><wl-source-bar [parts]="parts" [scale]="scale" [height]="height" /></div>`,
  }),
};
export default meta;
type Story = StoryObj<SourceBarComponent>;

export const Day: Story = {};
export const Overtime: Story = { args: { scale: 3600 } };
export const Empty: Story = { args: { parts: [] } };
