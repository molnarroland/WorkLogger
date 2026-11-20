import { ChangeDetectionStrategy, Component, effect, input, model, output, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  lucideActivity,
  lucideBookOpen,
  lucideCalendar,
  lucideClipboardList,
  lucideClock,
  lucideCloudDownload,
  lucideHardDrive,
  lucideMail,
  lucideMessagesSquare,
  lucideSettings,
  lucideUsers,
} from '@ng-icons/lucide';
import { SnAssistantImports, SnModalSheet, SnSettingsImports } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmRadioGroupImports } from '@spartan-ng/helm/radio-group';
import { HlmSwitchImports } from '@spartan-ng/helm/switch';
import type { Module } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import type { Step } from '../../shell/panes';
import type { SetupStep } from '../../shared/setup-steps';
import { SetupChecklistComponent } from '../setup-checklist/setup-checklist.component';

export const MODULES: { id: Module; icon: string }[] = [
  { id: 'git', icon: 'lucideHardDrive' },
  { id: 'calendar', icon: 'lucideUsers' },
  { id: 'workTimes', icon: 'lucideClock' },
  { id: 'chats', icon: 'lucideMessagesSquare' },
  { id: 'jira', icon: 'lucideActivity' },
  { id: 'confluence', icon: 'lucideBookOpen' },
  { id: 'email', icon: 'lucideMail' },
];

export type WelcomePage = 'intro' | 'sources' | 'tempo' | 'setup';
export const WELCOME_PAGES: WelcomePage[] = ['intro', 'sources', 'tempo', 'setup'];

/** The month's workflow, as the sidebar groups it. */
const FLOW: { step: Step; icon: string }[] = [
  { step: 'setup', icon: 'lucideSettings' },
  { step: 'gather', icon: 'lucideCloudDownload' },
  { step: 'fill', icon: 'lucideCalendar' },
  { step: 'finish', icon: 'lucideClipboardList' },
];

/**
 * The first-run guide (later behind Modules…): what WorkLogger does, the data sources, how worklogs get into
 * Tempo, then the accounts and folders those need, step by step.
 */
@Component({
  selector: 'wl-welcome-sheet',
  imports: [
    HlmButtonImports,
    HlmRadioGroupImports,
    HlmSwitchImports,
    NgIcon,
    SetupChecklistComponent,
    SnAssistantImports,
    SnModalSheet,
    SnSettingsImports,
    TPipe,
  ],
  providers: [
    provideIcons({
      lucideActivity,
      lucideBookOpen,
      lucideCalendar,
      lucideClipboardList,
      lucideClock,
      lucideCloudDownload,
      lucideHardDrive,
      lucideMail,
      lucideMessagesSquare,
      lucideSettings,
      lucideUsers,
    }),
  ],
  templateUrl: './welcome-sheet.component.html',
  styleUrl: './welcome-sheet.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class WelcomeSheetComponent {
  readonly open = model(false);
  readonly modules = model.required<Module[]>();
  readonly tempoByHand = model(true);
  /** What the chosen modules still need (setupSteps of the saved settings). */
  readonly steps = input<SetupStep[]>([]);
  /** The page to open on; Modules… opens on the sources. */
  readonly startPage = input<WelcomePage>('intro');
  /** A setup step's button: the host closes the sheet and takes the user there. */
  readonly act = output<SetupStep>();

  protected readonly all = MODULES;
  protected readonly flow = FLOW;
  /** The current page of the assistant, as an index into WELCOME_PAGES. */
  protected readonly step = signal(0);

  constructor() {
    // Every opening starts on its start page.
    effect(() => {
      if (this.open()) this.step.set(WELCOME_PAGES.indexOf(this.startPage()));
    });
  }

  protected toggle(id: Module, on: boolean) {
    const rest = this.modules().filter((m) => m !== id);
    this.modules.set(on ? [...rest, id] : rest);
  }

  protected setHow(value: unknown) {
    this.tempoByHand.set(value === 'hand');
  }
}
