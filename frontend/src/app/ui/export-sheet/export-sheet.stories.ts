import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn, screen } from 'storybook/test';
import { oct } from '../../api/testing/story-data';
import { ExportSheetComponent } from './export-sheet.component';

const meta: Meta<ExportSheetComponent> = {
  title: 'UI/Export sheet',
  component: ExportSheetComponent,
  args: {
    open: true,
    choices: oct.exports,
    from: '2026-10-01',
    to: '2026-10-31',
    copyMarkdown: fn(),
    save: fn(),
    markExported: fn(),
  },
};
export default meta;
type Story = StoryObj<ExportSheetComponent>;

export const October: Story = {};
export const Nothing: Story = { args: { choices: [] } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const MarksExported: Story = {
  play: async ({ args, userEvent }) => {
    await userEvent.click(await screen.findByRole('button', { name: /Mark as exported/ }));
    await expect(args.markExported).toHaveBeenCalled();
  },
};
