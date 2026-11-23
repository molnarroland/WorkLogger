import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  afterRenderEffect,
  computed,
  inject,
  input,
  resource,
  signal,
  untracked,
} from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  phosphorArrowCircleDown,
  phosphorClipboard,
  phosphorUserGear,
  phosphorChats,
  phosphorMagnifyingGlassMinus,
  phosphorTrash,
  phosphorUser,
  phosphorUsers,
} from '@ng-icons/phosphor-icons/regular';
import { Router } from '@angular/router';
import { SnCommandBarImports, SnModalSheet, SnPageCommandsTemplate, SnStatusPage } from '@spartan-native/desktop-ui';
import { SnSelectableImports } from '@spartan-native/helm';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmTextareaImports } from '@spartan-ng/helm/textarea';
import { API, EVENTS, type ChatDayDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { DurationPipe } from '../../shared/duration.pipe';
import { dayTitle, hm } from '../../shared/worklogs';
import { ShellStore } from '../../state/shell.store';
import { ChatMessagesComponent } from '../../ui/chat-messages/chat-messages.component';
import { clearFocus } from '../../shared/focus';
import { ScrollIntoViewDirective } from '../../ui/scroll-into-view.directive';
import { ListFilterComponent } from '../../ui/list-filter/list-filter.component';

/** Chat histories of the month: per day, each conversation with its message count and active time. */
@Component({
  selector: 'wl-chats-page',
  imports: [
    ListFilterComponent,
    ScrollIntoViewDirective,
    ChatMessagesComponent,
    DurationPipe,
    HlmButtonImports,
    HlmTextareaImports,
    NgIcon,
    SnPageCommandsTemplate,
    SnCommandBarImports,
    SnModalSheet,
    SnSelectableImports,
    SnStatusPage,
    TPipe,
  ],
  providers: [
    provideIcons({
      phosphorArrowCircleDown,
      phosphorClipboard,
      phosphorUserGear,
      phosphorChats,
      phosphorMagnifyingGlassMinus,
      phosphorTrash,
      phosphorUser,
      phosphorUsers,
    }),
  ],
  templateUrl: './chats-page.component.html',
  styleUrl: './chats-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ChatsPageComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);
  protected readonly selection = signal<readonly string[]>([]);
  protected readonly pasting = signal(false);
  protected readonly pasteText = signal('');
  protected readonly pasteError = signal('');
  protected readonly time = hm;

  protected readonly chats = resource({
    params: () => (this.store.range().from ? this.store.range() : undefined),
    loader: ({ params }) => this.api.chats(params.from, params.to),
  });
  private readonly router = inject(Router);
  private readonly selectedId = computed(() => this.selection()[0] ?? null);
  protected readonly messages = resource({
    params: () => this.selectedId() ?? undefined,
    loader: ({ params }) => this.api.chatMessages(params),
  });

  /** `?day=`: only that day's conversations (a link from a day's panel or an entry). */
  readonly day = input<string>();
  /** `?focus=`: the chat-day to open (its id, "day|conversation"). */
  readonly focus = input<string>();
  /** The search box: matched against the conversation's name and its issue keys. */
  protected readonly query = signal('');
  private readonly all = computed(() => this.chats.value()?.days ?? []);
  /** The month has chats at all (otherwise the page offers to paste some). */
  protected readonly hasAny = computed(() => this.all().some((d) => d.chats.length > 0));
  protected readonly filtered = computed(() => !!this.day() || !!this.query().trim());
  protected readonly total = computed(() => this.all().reduce((n, d) => n + d.chats.length, 0));
  /** The days shown: only `?day=` when set, each with the chats the search matches. */
  private readonly days = computed(() => {
    const day = this.day();
    const q = this.query().trim().toLocaleLowerCase();
    return this.all()
      .filter((d) => !day || d.day === day)
      .map((d) => {
        if (!q) return d;
        const chats = d.chats.filter((c) => chatMatches(c, q));
        return { ...d, chats, seconds: chats.reduce((n, c) => n + c.activeSeconds, 0) };
      })
      .filter((d) => d.chats.length > 0);
  });
  protected readonly list = computed(() => this.days().flatMap((d) => d.chats));
  protected readonly active = computed(() => this.chats.value()?.active ?? 0);
  protected readonly selected = computed(() => this.list().find((c) => c.id === this.selectedId()) ?? null);
  protected readonly ruleIssue = computed(() => this.selected()?.ruleIssue ?? null);
  /** The backend's days, with their titles in the UI language. */
  protected readonly byDay = computed(() =>
    this.days().map((d) => ({ ...d, title: dayTitle(d.day, this.i18n.lang()) })),
  );

  constructor() {
    // A link selects its chat once it is in the list, which shows its messages; after that the selection is the user's.
    // After render: the list drops a selected value that isn't one of its rendered items yet.
    let opened: string | undefined;
    afterRenderEffect(() => {
      const focus = this.focus();
      if (!focus || focus === opened || !this.list().some((c) => c.id === focus)) return;
      opened = focus;
      untracked(() => this.selection.set([focus]));
    });
    // Each fetched day shows as soon as it is saved, while the next one is read.
    inject(DestroyRef).onDestroy(this.api.onEvent<string>(EVENTS.chatDay, () => this.chats.reload()));
  }

  protected async log(c: ChatDayDto, issueKey: string) {
    await this.store.attempt(() => this.api.logChat(c.id, issueKey));
    await this.store.setMonth(this.store.month()); // the chat's worklog shows on the other pages too
    this.chats.reload();
  }

  protected async always(c: ChatDayDto, issueKey: string | null) {
    await this.store.editSettings((api) => api.setChatRule(c.conversationId, c.name, issueKey ?? ''));
    await this.store.run('rebuild');
    this.chats.reload();
  }

  protected openPaste() {
    this.pasteError.set('');
    this.pasting.set(true);
  }

  protected async importText() {
    try {
      const n = await this.api.importChats(this.pasteText());
      this.store.status.set(this.i18n.t('chats.imported', { n }));
      this.pasteText.set('');
      this.pasting.set(false);
      this.chats.reload();
    } catch (e) {
      this.pasteError.set(this.i18n.error(e));
    }
  }

  /** A day picked in the filter: `?day=` like a link from a day, so the two agree. */
  protected setDay(day: string | null) {
    return this.router.navigate([], { queryParams: { day, focus: null }, queryParamsHandling: 'merge' });
  }

  protected clearFilter() {
    this.query.set('');
    return clearFocus(this.router);
  }

  protected openProfiles() {
    this.router.navigate(['settings'], { queryParams: { tab: 'profiles' } });
  }

  protected async fetch() {
    await this.store.run('fetchGoogleChat');
    this.chats.reload();
  }

  protected async clear() {
    const { from, to } = this.store.range();
    await this.store.attempt(() => this.api.clearChats(from, to));
    this.selection.set([]);
    this.chats.reload();
  }
}

/** `q` is lower-case and trimmed. */
function chatMatches(c: ChatDayDto, q: string) {
  return [c.name, c.issueKey, c.ruleIssue].some((v) => !!v && v.toLocaleLowerCase().includes(q));
}
