import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideArrowUpRight, lucideMessagesSquare, lucideUsers } from '@ng-icons/lucide';
import type { ChatDayDto, Focus, MeetingDto } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import { hm } from '../../shared/worklogs';
import { SnTooltip } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';

export interface EvidenceItem {
  id: string;
  kind: 'meeting' | 'chat';
  title: string;
  from: string;
  to: string;
  /** Messages in a chat. */
  count: number;
  issueKey: string | null;
  skipped: boolean;
  /** Where it shows in full: the Meetings or Chats page, on its day. */
  focus: Focus;
}

/**
 * What was collected for a day besides the entries: its meetings and chats, in time order. Their tickets are
 * buttons: `use` hands the ticket to whatever the page is fixing (the selected entry, or the day's entries without
 * one). `canUse` false shows them as plain text.
 */
@Component({
  selector: 'wl-day-evidence',
  imports: [HlmButtonImports, SnTooltip, NgIcon, TPipe],
  providers: [provideIcons({ lucideArrowUpRight, lucideMessagesSquare, lucideUsers })],
  templateUrl: './day-evidence.component.html',
  styleUrl: './day-evidence.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DayEvidenceComponent {
  readonly meetings = input<MeetingDto[]>([]);
  readonly chats = input<ChatDayDto[]>([]);
  readonly canUse = input(false);
  readonly use = output<string>();
  /** The pages that are switched on (ids): an item can be opened on its page when that page is one of them. */
  readonly panes = input<readonly string[]>([]);
  /** Open a meeting or chat on its page, narrowed to its day. */
  readonly open = output<Focus>();

  protected readonly items = computed<EvidenceItem[]>(() =>
    [
      ...this.meetings().map((m): EvidenceItem => ({
        id: m.id,
        kind: 'meeting',
        title: m.title,
        from: m.start,
        to: m.end,
        count: 0,
        issueKey: m.issueKey,
        skipped: m.skipped || m.skippedByKeyword,
        focus: { pane: 'meetings', day: m.day, id: m.id },
      })),
      ...this.chats().map((c): EvidenceItem => ({
        id: c.id,
        kind: 'chat',
        title: c.name,
        from: c.first,
        to: c.last,
        count: c.count,
        issueKey: c.issueKey,
        skipped: false,
        focus: { pane: 'chats', day: c.day, id: c.id },
      })),
    ].sort((a, b) => a.from.localeCompare(b.from)),
  );

  protected readonly hm = hm;
}
