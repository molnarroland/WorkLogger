import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, fn } from 'storybook/test';
import type { ChatDayDto } from '../../api/api';
import { ChatMessagesComponent } from './chat-messages.component';

const chat: ChatDayDto = {
  id: 'c1',
  day: '2026-10-05',
  conversationId: 'space/orb',
  name: 'ORB',
  type: 'space',
  count: 4,
  activeSeconds: 1800,
  first: '2026-10-05T09:02:00Z',
  last: '2026-10-05T09:31:00Z',
  issueKey: null,
  exported: false,
  ruleIssue: null,
};

const meta: Meta<ChatMessagesComponent> = {
  title: 'UI/Chat messages',
  component: ChatMessagesComponent,
  args: {
    chat,
    ruleIssue: null,
    log: fn(),
    always: fn(),
    messages: [
      {
        time: '2026-10-05T09:02:00Z',
        sender: 'anna@company.com',
        me: false,
        text: 'Can you look at the sync job?',
        files: [],
      },
      { time: '2026-10-05T09:05:00Z', sender: '', me: true, text: 'On it.', files: [] },
      { time: '2026-10-05T09:31:00Z', sender: 'anna@company.com', me: false, text: '', files: ['trace.log'] },
    ],
  },
  render: (args) => ({
    props: args,
    template: `<wl-chat-messages style="height: 18rem" [chat]="chat" [messages]="messages" [ruleIssue]="ruleIssue" (log)="log($event)" (always)="always($event)" />`,
  }),
};
export default meta;
type Story = StoryObj<ChatMessagesComponent>;

export const Unticketed: Story = {};
export const WithRule: Story = { args: { ruleIssue: 'ORB-214', chat: { ...chat, issueKey: 'ORB-214' } } };
export const Hungarian: Story = { parameters: { lang: 'hu' } };
export const LogsTime: Story = {
  play: async ({ canvas, userEvent, args }) => {
    await userEvent.type(canvas.getByLabelText('Ticket, e.g. PROJ-12'), 'orb-9{Enter}');
    await expect(args.log).toHaveBeenCalledWith('orb-9'); // as typed: the backend normalises keys
  },
};
