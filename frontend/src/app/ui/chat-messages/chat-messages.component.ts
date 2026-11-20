import { ChangeDetectionStrategy, Component, computed, input, linkedSignal, output } from '@angular/core';
import { SnDialogActionsImports } from '@spartan-native/helm';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmCheckboxImports } from '@spartan-ng/helm/checkbox';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { HlmLabelImports } from '@spartan-ng/helm/label';
import type { ChatDayDto, ChatMessageDto } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { hm } from '../../shared/worklogs';
import { SnTooltip } from '@spartan-native/desktop-ui';

/** Chatting shorter than this isn't worth a worklog (ChatDay.minLoggable). */
const MIN_LOGGABLE = 60;

/** The selected conversation's messages, oldest first, with a ticket field to log its time and an "Always" rule. */
@Component({
  selector: 'wl-chat-messages',
  imports: [
    SnTooltip,
    DurationPipe,
    HlmButtonImports,
    HlmCheckboxImports,
    HlmInputImports,
    HlmLabelImports,
    SnDialogActionsImports,
    TPipe,
  ],
  templateUrl: './chat-messages.component.html',
  styleUrl: './chat-messages.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ChatMessagesComponent {
  readonly chat = input.required<ChatDayDto>();
  readonly messages = input<ChatMessageDto[]>([]);
  /** Ticket of the conversation's "Always" rule, if it has one. */
  readonly ruleIssue = input<string | null>(null);
  readonly log = output<string>();
  /** A ticket turns the rule on; null turns it off. */
  readonly always = output<string | null>();

  protected readonly minLoggable = MIN_LOGGABLE;
  protected readonly issue = linkedSignal(() => this.chat().issueKey ?? '');
  /** What was typed; the backend normalises it. */
  protected readonly key = computed(() => this.issue().trim() || null);
  protected readonly time = hm;

  protected who(m: ChatMessageDto) {
    return m.sender ? m.sender.split('@')[0] : '?';
  }

  protected submit() {
    const key = this.key();
    if (key && !this.chat().exported) this.log.emit(key);
  }
}
