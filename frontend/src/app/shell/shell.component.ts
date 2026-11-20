import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  resource,
  signal,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { NgTemplateOutlet } from '@angular/common';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  lucideActivity,
  lucideBookOpen,
  lucideMail,
  lucideCalendar,
  lucideCalendar1,
  lucideCalendarCheck,
  lucideCalendarRange,
  lucideCalendarClock,
  lucideChartBar,
  lucideChevronLeft,
  lucideChevronRight,
  lucideChevronsLeft,
  lucideChevronsRight,
  lucideCircleArrowDown,
  lucideCircleHelp,
  lucideCircleCheck,
  lucideCircleX,
  lucideClipboardList,
  lucideClock,
  lucideFlag,
  lucideHardDrive,
  lucideHouse,
  lucideInbox,
  lucideLayoutGrid,
  lucideListChecks,
  lucideLock,
  lucideMenu,
  lucideMessagesSquare,
  lucidePanelLeft,
  lucideSearch,
  lucideSettings,
  lucideShare,
  lucideTerminal,
  lucideUsers,
} from '@ng-icons/lucide';
import {
  SnCommandBarImports,
  SnMenuImports,
  SnPageCommands,
  SnPlatformImports,
  SnResizable,
  SnSidebarNavImports,
  SnStatusPage,
  SnTooltip,
  SnWindowImports,
} from '@spartan-native/desktop-ui';
import { SpartanNativeTheme, skinFamily } from '@spartan-native/theme';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import { filter, map } from 'rxjs';
import { API, type DayDto, type Module } from '../api/api';
import { I18n, TPipe, langFor, rememberLang } from '../i18n/t.pipe';
import { type Pane, PANES, STEPS } from './panes';
import { appMenu } from './app-menu';
import { ExportFlowComponent } from './export-flow.component';
import { type Shortcut, ShortcutService } from './shortcuts';
import { TitleFitDirective } from './title-fit.directive';
import { lookOf, lookOverrides, rememberLook } from '../shared/appearance';
import { DurationPipe } from '../shared/duration.pipe';
import { TRIAGE_COLOR } from '../shared/colors';
import { monthTitle } from '../shared/month';
import { formatDuration, parseDay } from '../shared/worklogs';
import { type Heading, ShellStore } from '../state/shell.store';
import { ActivityConsoleComponent } from '../ui/activity-console/activity-console.component';
import { TaskListComponent } from '../ui/task-list/task-list.component';
import { type DockItem, SourceDockComponent } from '../ui/source-dock/source-dock.component';
import { PeriodPickerComponent } from '../ui/period-picker/period-picker.component';
import { PageLauncherComponent } from '../ui/page-launcher/page-launcher.component';
import { OnboardingComponent } from './onboarding/onboarding.component';
import { type WelcomePage, WelcomeSheetComponent } from '../ui/welcome-sheet/welcome-sheet.component';
import { type SetupStep, setupLink, toSetupSteps } from '../shared/setup-steps';

/**
 * The window: title bar, sidebar navigation, the month's command bar, the current page and the status bar.
 * Everything visual comes from Spartan Native; shell.component.css only lays out.
 */
/** The pages the bottom dock lists, in its order (the backend's SourceCounts keys). */
const DOCK_PANES = ['repositories', 'meetings', 'sessions', 'chats', 'jira', 'confluence', 'email', 'periods'];
/** The finishing pages, in the dock's glass group at its right end. */
const DOCK_END = ['timesheet', 'review'];
/** The days a month step stands for on the outer pager buttons. */
const MONTH_DAYS = 30;

@Component({
  selector: 'wl-shell',
  imports: [
    SnTooltip,
    ActivityConsoleComponent,
    DurationPipe,
    ExportFlowComponent,
    HlmButtonImports,
    HlmSpinnerImports,
    NgIcon,
    NgTemplateOutlet,
    SnResizable,
    RouterLink,
    RouterLinkActive,
    RouterOutlet,
    SnCommandBarImports,
    SnMenuImports,
    SnPlatformImports,
    SnSidebarNavImports,
    SnStatusPage,
    SnWindowImports,
    PageLauncherComponent,
    PeriodPickerComponent,
    SourceDockComponent,
    TaskListComponent,
    TitleFitDirective,
    TPipe,
    WelcomeSheetComponent,
    OnboardingComponent,
  ],
  providers: [
    provideIcons({
      lucideActivity,
      lucideBookOpen,
      lucideMail,
      lucideCalendar,
      lucideCalendar1,
      lucideCalendarCheck,
      lucideCalendarRange,
      lucideCalendarClock,
      lucideChartBar,
      lucideChevronLeft,
      lucideChevronRight,
      lucideChevronsLeft,
      lucideChevronsRight,
      lucideCircleArrowDown,
      lucideCircleHelp,
      lucideCircleCheck,
      lucideCircleX,
      lucideClipboardList,
      lucideClock,
      lucideFlag,
      lucideHardDrive,
      lucideHouse,
      lucideInbox,
      lucideLayoutGrid,
      lucideListChecks,
      lucideLock,
      lucideMenu,
      lucideMessagesSquare,
      lucidePanelLeft,
      lucideSearch,
      lucideSettings,
      lucideShare,
      lucideTerminal,
      lucideUsers,
    }),
  ],
  templateUrl: './shell.component.html',
  styleUrl: './shell.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ShellComponent {
  protected readonly store = inject(ShellStore);
  protected readonly pageCommands = inject(SnPageCommands);
  /** The status bar's Tasks badge: a spinner while something runs, else how many failed. */
  protected readonly runningTasks = computed(() => this.store.tasks().some((t) => t.state === 'running'));
  protected readonly failedTasks = computed(() => this.store.tasks().filter((t) => t.state === 'failed').length);
  protected readonly theme = inject(SpartanNativeTheme);
  private readonly router = inject(Router);
  private readonly i18n = inject(I18n);

  /**
   * The real OS, not the skin: Settings → General can dress the window as another OS, but the window buttons,
   * the global menu bar and the shortcut keys still belong to the one it runs on.
   */
  protected readonly family = computed(() => skinFamily(this.theme.platform()?.skin ?? this.theme.skin()));
  /** The family the window is dressed as (Settings → General → Look like). */
  protected readonly skinFamily = computed(() => skinFamily(this.theme.skin()));

  /**
   * Who draws the window buttons, or null for no title bar:
   *  - Wails on Windows / Linux: frameless window, the title bar draws them and they act on it ('custom')
   *  - Wails on macOS: hidden-inset title bar, the OS keeps the traffic lights ('native')
   *  - a plain browser (ng serve, Storybook): the browser owns the window (null)
   */
  protected readonly windowControls = computed<'custom' | 'native' | null>(() => {
    if (this.theme.runtime !== 'wails') return null;
    return this.family() === 'macos' ? 'native' : 'custom';
  });

  /** macOS in Wails: the menu is the system's global menu bar, which also owns the shortcuts. */
  protected readonly nativeMenubar = computed(
    () => this.theme.runtime === 'wails' && this.family() === 'macos' && !!this.theme.menus?.setApplicationMenu,
  );

  private readonly modulesOpen = signal(false);
  /** Where the guide opens: the start on first run and from "Setup guide…", the sources from "Modules…". */
  protected readonly welcomePage = signal<WelcomePage>('intro');
  /** What the chosen modules still need: the guide's last page and Settings → Profiles list it. */
  protected readonly setupSteps = computed(() => toSetupSteps(this.store.setup()));
  /** From "Guide": the first run has the onboarding instead. */
  protected readonly welcomeOpen = this.modulesOpen.asReadonly();
  protected readonly exportOpen = signal(false);
  protected readonly consoleOpen = signal(false);

  protected readonly range = this.store.range;
  protected readonly monthTitle = computed(() => monthTitle(this.store.month(), this.i18n.lang()));
  protected readonly isCurrentMonth = computed(() => this.store.month() === this.store.currentMonth());
  /** The side panel's top: Overview, Days, Week. */
  protected readonly views = computed(() => this.store.panes().filter((p) => p.view));
  /** Every other page, by workflow step, under "All pages". */
  protected readonly moreGroups = computed(() =>
    STEPS.map((step) => ({ step, panes: this.store.panes().filter((p) => !p.view && p.step === step) })).filter(
      (g) => g.panes.length > 0,
    ),
  );

  private readonly url = toSignal(
    this.router.events.pipe(
      filter((e) => e instanceof NavigationEnd),
      map(() => this.router.url),
    ),
    { initialValue: this.router.url },
  );
  protected readonly pane = computed(() => this.url().split(/[/?#]/)[1] ?? '');
  /** The pages pinned to the side panel (ids, in the order pinned); remembered in this window. */
  protected readonly pinned = signal<string[]>(readList(PINNED_KEY));
  /** The pinned pages that are on (their module may be off), in workflow order. */
  protected readonly pinnedPanes = computed(() =>
    this.store.panes().filter((p) => !p.view && this.pinned().includes(p.id)),
  );
  /** "All pages" stands for the open page when it is one of them and not pinned: its name and icon, as current. */
  protected readonly morePane = computed(
    () => this.store.panes().find((p) => !p.view && p.id === this.pane() && !this.pinned().includes(p.id)) ?? null,
  );
  /** The day open in the Days view, to mark it in the side panel's lists. */
  protected readonly openDay = computed(() => /^\/days\/(\d{4}-\d{2}-\d{2})/.exec(this.url())?.[1] ?? null);

  private readonly api = inject(API);
  /** The day the dock counts and opens: the Days view's day, or a source page's `?day=`; none means the month. */
  protected readonly scopeDay = computed(
    () => this.openDay() ?? /[?&]day=(\d{4}-\d{2}-\d{2})/.exec(this.url())?.[1] ?? null,
  );
  /** The days the dock counts: the scoped day, or the month. */
  private readonly scope = computed(() => {
    const { from, to } = this.store.range();
    const day = this.scopeDay();
    return { from: day ?? from, to: day ?? to };
  });
  /** Re-counted when the day changes or the month is re-read (after a collect, fetch or rebuild). */
  private readonly counts = resource({
    params: () => {
      if (!this.scope().from || this.store.locked()) return undefined;
      return { ...this.scope(), data: this.store.data(), modules: this.store.modules() };
    },
    loader: ({ params }) => this.api.sourceCounts(params.from, params.to),
  });
  /** What the month's data already knows: periods over the days, days still to do, drafts to review. */
  private localCount(id: string): number {
    const { from, to } = this.scope();
    const inScope = (day: string) => day >= from && day <= to;
    switch (id) {
      case 'periods':
        return (this.store.settings()?.periods ?? []).filter((p) => p.issueKey && p.from <= to && p.to >= from).length;
      case 'timesheet':
        return [...this.store.days().values()].filter(
          (d) => inScope(d.day) && ['empty', 'partial', 'noTicket', 'draft'].includes(d.state),
        ).length;
      case 'review':
        return this.store.worklogs().filter((w) => inScope(w.day) && w.status === 'draft').length;
    }
    return 0;
  }
  /** The source pages whose module is on, then the finishing pages; each with its count. */
  protected readonly dock = computed<DockItem[]>(() => {
    const counts = this.counts.value();
    const on = this.store.paneIds();
    const shown = (id: string) => on.includes(id) && (id !== 'jira' || this.store.modules().includes('jira'));
    return [...DOCK_PANES, ...DOCK_END].flatMap((id) => {
      const p = PANES.find((x) => x.id === id);
      if (!p || !shown(id)) return [];
      const count = counts && id in counts ? counts[id as keyof typeof counts] : this.localCount(id);
      return [{ id, label: p.label, icon: p.icon, count, end: DOCK_END.includes(id) }];
    });
  });

  /** The calendar button: the page's own today (this day, this week), else this month. */
  protected readonly today = computed(() => {
    const t = this.store.pageHeader()?.today;
    if (t) return { label: t.label, icon: t.icon, current: t.current(), go: t.go };
    return {
      label: 'month.this',
      icon: 'lucideCalendarCheck',
      current: this.isCurrentMonth(),
      go: () => this.thisMonth(),
    };
  });
  protected readonly monthDays = MONTH_DAYS;
  /** The calendar after the pager: the page's day or week, else the month. */
  protected readonly picker = computed(() => {
    const p = this.store.pageHeader()?.pick;
    if (p) return { label: p.label, mode: p.mode, value: p.value(), go: p.go };
    return {
      label: 'month.pick',
      mode: 'month' as const,
      value: this.store.month(),
      go: (m: string) => this.store.setMonth(m),
    };
  });
  protected readonly lang = computed(() => this.i18n.lang());

  /** Opens the calendar under its button (a popover sits in the top layer, so it is placed by hand). */
  protected placePicker(e: Event) {
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const pop = document.getElementById('wl-pick');
    if (pop) Object.assign(pop.style, { top: `${r.bottom + 4}px`, left: `${r.left}px` });
  }

  protected picked(value: string) {
    void this.picker().go(value);
    document.getElementById('wl-pick')?.hidePopover();
  }

  /** A dock button: the page, narrowed to the counted day when there is one. */
  protected openSource(id: string) {
    return this.router.navigate(['/' + id], { queryParams: { day: this.scopeDay() } });
  }

  /**
   * The side panel folded to icons: the views, then a numbered chip per day. Remembered between starts; a narrow
   * window always folds it.
   */
  protected readonly collapsed = signal(readFlag(COLLAPSED_KEY));
  protected readonly narrow = signal(false);
  protected readonly folded = computed(() => this.collapsed() || this.narrow());
  /** The fold button's icon: macOS's sidebar toggle, the navigation pane's hamburger elsewhere. */
  protected readonly toggleIcon = computed(() => (this.skinFamily() === 'macos' ? 'lucidePanelLeft' : 'lucideMenu'));
  protected readonly triageColor = TRIAGE_COLOR;
  protected readonly dayGroups = computed(() => {
    return [
      { id: 'attention', title: 'side.attention', days: this.store.attention() },
      { id: 'drafts', title: 'side.drafts', days: this.store.drafts() },
    ].filter((g) => g.days.length > 0);
  });

  /**
   * The header line's centre: the page's own heading (the Week page's week), else the month's, with their logged
   * and expected time; null on Settings, which shows its name (pageTitle).
   */
  protected readonly heading = computed<Heading | null>(() => {
    if (this.pane() === 'settings') return null;
    const own = this.store.pageHeader()?.heading?.();
    if (own) return own;
    const s = this.store.summary();
    return s ? { title: this.monthTitle(), logged: s.logged, expected: s.expected } : null;
  });

  /** The heading bar's fill: logged as a share of expected, in percent (full when over). */
  protected share(h: Heading) {
    return h.expected > 0 ? Math.min(100, (h.logged / h.expected) * 100) : h.logged > 0 ? 100 : 0;
  }

  /** The header line's title when there is no heading: the open page's name (an i18n key). */
  protected readonly pageTitle = computed(() => {
    const id = this.pane();
    if (id === 'settings') return 'pane.settings';
    return this.store.panes().find((p) => p.id === id)?.label ?? 'app.name';
  });

  /** The macOS menu bar; the same commands as the command bar, with their shortcuts. */
  protected readonly menu = computed(() =>
    appMenu({
      locked: this.store.locked(),
      busy: this.store.busy(),
      modules: this.store.modules(),
      panes: this.store.panes(),
      isCurrentMonth: this.isCurrentMonth(),
      t: (k) => this.i18n.t(k),
      run: (job) => this.store.run(job),
      go: (pane) => this.go(pane),
      shiftMonth: (by) => this.shift(by),
      thisMonth: () => this.thisMonth(),
      openExport: () => this.exportOpen.set(true),
      openGuide: () => this.openGuide(),
      openModules: () => this.openModules(),
    }),
  );

  constructor() {
    // Elsewhere the shortcuts are bound in the page (on macOS-in-Wails the global menu bar binds them).
    const off = inject(ShortcutService).listen(
      () => this.family(),
      (s) => !this.nativeMenubar() && this.onShortcut(s),
    );
    inject(DestroyRef).onDestroy(off);

    if (typeof matchMedia === 'function') {
      const query = matchMedia('(max-width: 44rem)');
      const update = () => this.narrow.set(query.matches);
      update();
      query.addEventListener('change', update);
      inject(DestroyRef).onDestroy(() => query.removeEventListener('change', update));
    }

    // Settings → General: the user's language and look on top of the OS values, remembered for the next start.
    effect(() => {
      const s = this.store.settings();
      if (!s) return;
      rememberLang(s.language);
      const lang = langFor(s.language);
      this.i18n.lang.set(lang);
      document.documentElement.lang = lang;
      const look = lookOf(s);
      rememberLook(look);
      const overrides = lookOverrides(look);
      this.theme.followSystem();
      if (Object.keys(overrides).length) this.theme.override(overrides);
    });
  }

  protected badge(pane: Pane): string | number {
    if (pane.id === 'days') return this.store.attention().length || '';
    if (pane.id === 'review') return this.store.needsTicket() || '';
    return '';
  }

  protected togglePin(id: string) {
    const list = this.pinned().includes(id) ? this.pinned().filter((p) => p !== id) : [...this.pinned(), id];
    this.pinned.set(list);
    writeList(PINNED_KEY, list);
  }

  protected toggleSidebar() {
    this.collapsed.update((c) => !c);
    writeFlag(COLLAPSED_KEY, this.collapsed());
  }

  /** "Tue, Oct 27" / "okt. 27., K" */
  protected dayName(day: string) {
    return new Intl.DateTimeFormat(this.i18n.lang(), { weekday: 'short', month: 'short', day: 'numeric' }).format(
      parseDay(day),
    );
  }

  protected dayNumber(day: string) {
    return parseDay(day).getDate();
  }

  /** The line under a day in the side panel: what it still needs. */
  protected dayNote(d: DayDto) {
    switch (d.state) {
      case 'empty':
        return this.i18n.t('side.nothingLogged');
      case 'partial':
        return this.i18n.t('side.short', { d: formatDuration(d.short) });
      case 'noTicket':
        return this.i18n.t('day.withoutIssue', { n: d.missing });
      default:
        return this.i18n.t('side.toReview', { n: d.drafts });
    }
  }

  /** A day's whole description, for the folded chips and screen readers. */
  protected dayLabel(d: DayDto) {
    const date = new Intl.DateTimeFormat(this.i18n.lang(), { weekday: 'long', month: 'long', day: 'numeric' }).format(
      parseDay(d.day),
    );
    return `${date}, ${this.i18n.t('timesheet.loggedOf', { logged: formatDuration(d.logged), expected: formatDuration(d.expected) })}, ${this.dayNote(d)}`;
  }

  protected go(pane: string) {
    this.router.navigate([pane]);
  }

  protected shift(by: number) {
    const m = this.store.data();
    if (m) return this.store.setMonth(by < 0 ? m.prev : m.next);
    return undefined;
  }

  protected thisMonth() {
    return this.store.setMonth(this.store.currentMonth());
  }

  protected openModules() {
    return this.router.navigate(['settings'], { queryParams: { tab: 'modules' } });
  }

  protected openGuide() {
    this.welcomePage.set('intro');
    this.modulesOpen.set(true);
  }

  /** A setup step's button in the guide: close it and go where the step is done. */
  protected async openSetupStep(step: SetupStep) {
    this.finishWelcome();
    const { commands, queryParams } = setupLink(step.action);
    this.router.navigate(commands, { queryParams });
  }

  protected setModules(modules: Module[]) {
    this.store.saveSettings({ modules });
    // A page whose module was switched off is gone from the sidebar: go to the Timesheet.
    const visible = this.store.panes().some((p) => p.id === this.pane());
    if (!visible && this.pane() !== 'settings') this.go('overview');
  }

  protected finishWelcome() {
    this.modulesOpen.set(false);
  }

  private onShortcut(s: Shortcut) {
    if (this.store.locked()) return;
    switch (s.kind) {
      case 'page': {
        const pane = this.store.panes()[s.index];
        return pane && this.go(pane.id);
      }
      case 'prevMonth':
      case 'nextMonth':
        return this.shift(s.kind === 'nextMonth' ? 1 : -1);
      case 'logInTempo':
        return this.go('by-ticket');
      default:
        return this.store.run(s.kind);
    }
  }
}

const COLLAPSED_KEY = 'wl.sidebar.folded';
const PINNED_KEY = 'wl.sidebar.pinned';

/** Like readFlag: a list of strings, or none when it is missing, blocked or not a list. */
function readList(key: string): string[] {
  try {
    const v: unknown = JSON.parse(localStorage.getItem(key) ?? '[]');
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : [];
  } catch {
    return [];
  }
}

function writeList(key: string, list: string[]) {
  try {
    localStorage.setItem(key, JSON.stringify(list));
  } catch {
    // Not remembered; the pins still show until the window closes.
  }
}

/** A per-window convenience: storage may be missing or blocked (private windows, previews), so it never throws. */
function readFlag(key: string): boolean {
  try {
    return localStorage.getItem(key) === '1';
  } catch {
    return false;
  }
}

function writeFlag(key: string, on: boolean) {
  try {
    localStorage.setItem(key, on ? '1' : '0');
  } catch {
    // Not remembered; the panel still folds.
  }
}
