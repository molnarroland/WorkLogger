import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  computed,
  effect,
  inject,
  resource,
  signal,
  untracked,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { NgTemplateOutlet } from '@angular/common';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  phosphorPulse,
  phosphorBookOpen,
  phosphorEnvelopeSimple,
  phosphorCalendarBlank,
  phosphorCalendarDot,
  phosphorCalendarCheck,
  phosphorCalendarDots,
  phosphorCalendarPlus,
  phosphorChartBar,
  phosphorCaretLeft,
  phosphorCaretRight,
  phosphorCaretDoubleLeft,
  phosphorCaretDoubleRight,
  phosphorArrowCircleDown,
  phosphorQuestion,
  phosphorWarning,
  phosphorCheckCircle,
  phosphorXCircle,
  phosphorClipboardText,
  phosphorClock,
  phosphorFlag,
  phosphorHardDrive,
  phosphorHouse,
  phosphorTray,
  phosphorListChecks,
  phosphorLock,
  phosphorList,
  phosphorChats,
  phosphorSidebar,
  phosphorMagnifyingGlass,
  phosphorGear,
  phosphorExport,
  phosphorTerminalWindow,
  phosphorUsers,
} from '@ng-icons/phosphor-icons/regular';
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
import { HlmAlertImports } from '@spartan-ng/helm/alert';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import { HlmToggleGroupImports } from '@spartan-ng/helm/toggle-group';
import { filter, map } from 'rxjs';
import { API, type DayDto, type Module } from '../api/api';
import { I18n, TPipe, langFor, rememberLang } from '../i18n/t.pipe';
import { type Pane, PANES, type SideTab, TAB_OF } from './panes';
import { SIDE_ICONS, sideIcon } from './side-icons';
import { appMenu } from './app-menu';
import { ExportFlowComponent } from './export-flow.component';
import { type Shortcut, ShortcutService } from './shortcuts';
import { TitleFitDirective } from './title-fit.directive';
import { lookOf, lookOverrides, rememberLook } from '../shared/appearance';
import { DurationPipe } from '../shared/duration.pipe';
import { TRIAGE_COLOR } from '../shared/colors';
import { monthTitle } from '../shared/month';
import { dayNote, formatDuration, parseDay } from '../shared/worklogs';
import { type Heading, ShellStore } from '../state/shell.store';
import { ActivityConsoleComponent } from '../ui/activity-console/activity-console.component';
import { TaskListComponent } from '../ui/task-list/task-list.component';
import { type DockItem, SourceDockComponent } from '../ui/source-dock/source-dock.component';
import { PeriodPickerComponent } from '../ui/period-picker/period-picker.component';
import { OnboardingComponent } from './onboarding/onboarding.component';
import { SourceSetupComponent } from './source-setup/source-setup.component';
import { type WelcomePage, WelcomeSheetComponent } from '../ui/welcome-sheet/welcome-sheet.component';
import { SETUP_PANE, type SetupStep, setupLink, toSetupSteps } from '../shared/setup-steps';

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
    HlmAlertImports,
    HlmSpinnerImports,
    HlmToggleGroupImports,
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
    PeriodPickerComponent,
    SourceDockComponent,
    TaskListComponent,
    TitleFitDirective,
    TPipe,
    WelcomeSheetComponent,
    OnboardingComponent,
    SourceSetupComponent,
  ],
  providers: [
    provideIcons({
      ...SIDE_ICONS,
      phosphorPulse,
      phosphorBookOpen,
      phosphorEnvelopeSimple,
      phosphorCalendarBlank,
      phosphorCalendarDot,
      phosphorCalendarCheck,
      phosphorCalendarDots,
      phosphorCalendarPlus,
      phosphorChartBar,
      phosphorCaretLeft,
      phosphorCaretRight,
      phosphorCaretDoubleLeft,
      phosphorCaretDoubleRight,
      phosphorArrowCircleDown,
      phosphorQuestion,
      phosphorWarning,
      phosphorCheckCircle,
      phosphorXCircle,
      phosphorClipboardText,
      phosphorClock,
      phosphorFlag,
      phosphorHardDrive,
      phosphorHouse,
      phosphorTray,
      phosphorListChecks,
      phosphorLock,
      phosphorList,
      phosphorChats,
      phosphorSidebar,
      phosphorMagnifyingGlass,
      phosphorGear,
      phosphorExport,
      phosphorTerminalWindow,
      phosphorUsers,
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
  /** The side panel's groups: the Days tab's top and Month, the Sources tab's sources and calendar. */
  protected readonly views = computed(() => this.store.panes().filter((p) => p.place === 'view'));
  protected readonly monthPanes = computed(() => this.store.panes().filter((p) => p.place === 'month' && !p.parent));
  protected readonly sourcePanes = computed(() =>
    this.store
      .panes()
      .filter((p) => p.place === 'source' && (p.id !== 'jira' || this.store.modules().includes('jira'))),
  );
  protected readonly calendarPanes = computed(() => this.store.panes().filter((p) => p.place === 'calendar'));
  /** The side panel's tab; remembered in this window. */
  protected readonly sideTab = signal<SideTab>(readTab());
  /** The setup steps still to do: the Sources tab's count and its Finish setup item. */
  protected readonly setupLeft = computed(() => this.setupSteps().filter((s) => !s.done).length);
  /** The source pages with a setup step still to do. */
  private readonly needsSetup = computed(
    () => new Set(this.setupSteps().flatMap((s) => (!s.done && SETUP_PANE[s.id] ? [SETUP_PANE[s.id]] : []))),
  );

  private readonly url = toSignal(
    this.router.events.pipe(
      filter((e) => e instanceof NavigationEnd),
      map(() => this.router.url),
    ),
    { initialValue: this.router.url },
  );
  protected readonly pane = computed(() => this.url().split(/[/?#]/)[1] ?? '');
  /** The side panel item the open page belongs to: itself, or the page it is a tab of. */
  protected readonly paneGroup = computed(() => PANES.find((p) => p.id === this.pane())?.parent ?? this.pane());
  /** A page with tabs of its own pages (Log in Tempo: By ticket, Totals): the pages, else null. */
  protected readonly groupTabs = computed(() => {
    const kids = this.store.panes().filter((p) => p.parent === this.paneGroup());
    const root = PANES.find((p) => p.id === this.paneGroup());
    return root && kids.length ? [root, ...kids] : null;
  });
  /** A source page: it has Activity and Setup tabs (`?tab=setup`). */
  protected readonly isSource = computed(() => PANES.find((p) => p.id === this.pane())?.place === 'source');
  protected readonly sourceTab = computed(() =>
    this.isSource() && /[?&]tab=setup(&|#|$)/.test(this.url()) ? 'setup' : 'activity',
  );
  /** The open source's first setup step left, shown above its Activity tab. */
  protected readonly sourceStepLeft = computed(() =>
    this.isSource() ? (this.setupSteps().find((s) => !s.done && SETUP_PANE[s.id] === this.pane()) ?? null) : null,
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
  /** The month's counts (the side panel's sources); re-counted when the month is re-read (collect, fetch, rebuild). */
  private readonly monthCounts = resource({
    params: () => {
      const { from, to } = this.store.range();
      if (!from || this.store.locked()) return undefined;
      return { from, to, data: this.store.data(), modules: this.store.modules() };
    },
    loader: ({ params }) => this.api.sourceCounts(params.from, params.to),
  });
  /** The scoped day's counts, for the dock; the month's when no day is scoped. */
  private readonly dayCounts = resource({
    params: () => {
      const day = this.scopeDay();
      if (!day || this.store.locked()) return undefined;
      return { day, data: this.store.data(), modules: this.store.modules() };
    },
    loader: ({ params }) => this.api.sourceCounts(params.day, params.day),
  });
  private readonly counts = computed(() => (this.scopeDay() ? this.dayCounts : this.monthCounts));
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
    const counts = this.counts().value();
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
      icon: 'phosphorCalendarCheck',
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
  protected readonly toggleIcon = computed(() => (this.skinFamily() === 'macos' ? 'phosphorSidebar' : 'phosphorList'));
  protected readonly triageColor = TRIAGE_COLOR;
  protected readonly sideIcon = sideIcon;
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

    // Opening a page shows the tab that lists it, so the current page is always in view.
    effect(() => {
      const place = PANES.find((p) => p.id === this.pane())?.place;
      if (place) untracked(() => this.sideTab() !== TAB_OF[place] && this.setSideTab(TAB_OF[place]));
    });

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

  /** A source's badge: "Set up" (folded: "!") while a setup step is left, else its count for the month. */
  protected sourceBadge(pane: Pane): string | number {
    if (this.needsSetup().has(pane.id)) return this.folded() ? '!' : this.i18n.t('side.setUp');
    const counts = this.monthCounts.value();
    return (counts && pane.id in counts ? counts[pane.id as keyof typeof counts] : 0) || '';
  }

  protected pickSideTab(value: unknown) {
    if (value === 'days' || value === 'sources') this.setSideTab(value);
  }

  private setSideTab(tab: SideTab) {
    this.sideTab.set(tab);
    writeTab(tab);
  }

  protected pickGroupTab(value: unknown) {
    if (typeof value === 'string' && this.groupTabs()?.some((p) => p.id === value)) this.go(value);
  }

  protected pickSourceTab(value: unknown) {
    if (value !== 'setup' && value !== 'activity') return;
    void this.router.navigate([], {
      queryParams: { tab: value === 'setup' ? 'setup' : null },
      queryParamsHandling: 'merge',
    });
  }

  /** Finish setup: the guide, at its setup page. */
  protected openSetup() {
    this.welcomePage.set('setup');
    this.modulesOpen.set(true);
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

  protected dayNote(d: DayDto) {
    return dayNote(d, (k, p) => this.i18n.t(k, p));
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
const TAB_KEY = 'wl.sidebar.tab';

/** Like readFlag: the side panel's tab, Days when it is missing or blocked. */
function readTab(): SideTab {
  try {
    return localStorage.getItem(TAB_KEY) === 'sources' ? 'sources' : 'days';
  } catch {
    return 'days';
  }
}

function writeTab(tab: SideTab) {
  try {
    localStorage.setItem(TAB_KEY, tab);
  } catch {
    // Not remembered; the tab still switches.
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
