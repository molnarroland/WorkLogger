import { ChangeDetectionStrategy, Component, computed, inject, input, output } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideArrowUpRight, lucideTrash2 } from '@ng-icons/lucide';
import { SnInspectorImports, SnTooltip } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { HlmLabelImports } from '@spartan-ng/helm/label';
import { HlmSwitchImports } from '@spartan-ng/helm/switch';
import { HlmTextareaImports } from '@spartan-ng/helm/textarea';
import { HlmToggleGroupImports } from '@spartan-ng/helm/toggle-group';
import type { Focus, Status, WorklogDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { at, dayTitle, formatDuration, hm } from '../../shared/worklogs';

/**
 * The selected entry's fields, inside an `<sn-inspector>`. Each field commits on Enter or when it loses focus and
 * emits the whole entry; the backend locks an edited entry so a rebuild keeps the correction.
 */
@Component({
  selector: 'wl-worklog-inspector',
  imports: [
    SnTooltip,
    DurationPipe,
    HlmButtonImports,
    HlmInputImports,
    HlmLabelImports,
    HlmSwitchImports,
    HlmTextareaImports,
    HlmToggleGroupImports,
    NgIcon,
    SnInspectorImports,
    TPipe,
  ],
  providers: [provideIcons({ lucideArrowUpRight, lucideTrash2 })],
  templateUrl: './worklog-inspector.component.html',
  styleUrl: './worklog-inspector.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WorklogInspectorComponent {
  private readonly i18n = inject(I18n);
  readonly worklog = input.required<WorklogDto>();
  readonly changed = output<WorklogDto>();
  /** A duration as typed ("1h 30m", "1,5h", "90"); the backend parses it. */
  readonly duration = output<{ id: number; text: string }>();
  readonly remove = output<number>();
  /** The pages that are switched on (ids); the entry's source is offered only when its page is one of them. */
  readonly panes = input<readonly string[]>([]);
  /** "Show in Meetings": open where the entry came from. */
  readonly open = output<Focus>();
  /** A meeting's ticket for every meeting with its title: the host adds the rule and rebuilds. */
  readonly always = output<{ title: string; issueKey: string }>();

  protected readonly statuses: Status[] = ['draft', 'reviewed', 'exported'];
  protected readonly w = this.worklog;
  protected readonly time = computed(() => hm(this.w().start));
  protected readonly day = computed(() => dayTitle(this.w().day, this.i18n.lang()));
  /** Exported when the ticket's time for the day is already in Tempo (sent, ticked off, or found by read-back). */
  protected readonly status = computed(() => this.w().effectiveStatus);
  /** Sent through the Tempo API: only the lock and WorkLogger's own note may change (the backend enforces it). */
  protected readonly sent = computed(() => !this.w().canDelete);
  protected readonly id = computed(() => `wl-${this.w().id}`);
  /** Reviewed needs the comment for Tempo. */
  protected readonly noComment = computed(() => this.w().needsComment);
  /** A meeting with a ticket can make it the rule for its title (the description is the meeting's title). */
  protected readonly meetingRule = computed(() => {
    const w = this.w();
    const title = w.description.trim();
    return w.source === 'meeting' && w.issueKey && title && !this.sent() ? { title, issueKey: w.issueKey } : null;
  });
  protected readonly focus = computed(() => {
    const f = this.w().focus;
    return f && this.panes().includes(f.pane) ? f : null;
  });

  protected setStart(value: string) {
    if (value && value !== this.time()) this.emit({ start: at(this.w().day, value) });
  }

  protected setDuration(el: HTMLInputElement) {
    const text = el.value.trim();
    if (text && text !== this.durationText()) this.duration.emit({ id: this.w().id, text });
    el.value = this.durationText(); // the saved value comes back with the entry
  }

  /** What was typed; the backend normalises it (empty: no issue). */
  protected setIssue(el: HTMLInputElement) {
    const key = el.value.trim() || null;
    if (key !== this.w().issueKey) this.emit({ issueKey: key });
  }

  protected setDescription(value: string) {
    if (value !== this.w().description) this.emit({ description: value });
  }

  protected setComment(value: string) {
    const c = value.trim();
    if (c !== this.w().comment) this.emit({ comment: c });
  }

  protected setStatus(value: unknown) {
    if (typeof value === 'string' && value !== this.w().status) this.emit({ status: value as Status });
  }

  protected durationText() {
    return formatDuration(this.w().duration);
  }

  protected emit(patch: Partial<WorklogDto>) {
    this.changed.emit({ ...this.w(), ...patch });
  }
}
