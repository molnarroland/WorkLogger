import { ChangeDetectionStrategy, Component, computed, inject, resource } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideCalendar, lucideRss } from '@ng-icons/lucide';
import { SnSettingsImports } from '@spartan-native/desktop-ui';
import { HlmAccordionImports } from '@spartan-ng/helm/accordion';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { API, type SettingsDto } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import { ShellStore } from '../../state/shell.store';
import { ChipListComponent } from '../../ui/chip-list/chip-list.component';
import { type RuleItem, RuleListComponent } from '../../ui/rule-list/rule-list.component';

/**
 * Settings → Calendars: where meetings come from (the Google Calendar API, or calendar.google.com through the
 * Google profiles; ICS feeds) and how they are logged (default issue, never-log keywords, rules).
 */
@Component({
  selector: 'wl-calendar-settings',
  imports: [
    ChipListComponent,
    HlmAccordionImports,
    HlmBadgeImports,
    HlmButtonImports,
    HlmInputImports,
    NgIcon,
    RuleListComponent,
    SnSettingsImports,
    TPipe,
  ],
  providers: [provideIcons({ lucideCalendar, lucideRss })],
  templateUrl: './calendar-settings.component.html',
  styleUrl: './calendar-settings.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarSettingsComponent {
  protected readonly store = inject(ShellStore);
  protected readonly api = inject(API);

  protected readonly status = resource({ loader: () => this.api.calendarStatus() });
  /** Whether the Google client secret is saved (its value never comes back). */
  protected readonly secretSaved = resource({ loader: () => this.api.hasGoogleClientSecret() });

  protected async setSecret(e: Event) {
    const el = e.target as HTMLInputElement;
    try {
      await this.api.setGoogleClientSecret(el.value);
    } catch (err) {
      this.store.report(err);
    }
    el.value = '';
    this.secretSaved.reload();
  }
  protected readonly googleProfiles = computed(() =>
    (this.store.settings()?.profiles ?? []).filter((p) => p.kind === 'googleChat'),
  );
  protected readonly rules = computed<RuleItem[]>(() =>
    (this.store.settings()?.meetingRules ?? []).map((r) => ({ label: r.contains, issueKey: r.issueKey })),
  );

  protected save(patch: Partial<SettingsDto>) {
    return this.store.saveSettings(patch);
  }

  protected value(e: Event) {
    return (e.target as HTMLInputElement).value.trim();
  }

  protected addRule(r: RuleItem) {
    return this.store.editSettings((api) => api.addMeetingRule(r.label, r.issueKey ?? ''));
  }

  protected removeRule(i: number) {
    const id = this.store.settings()?.meetingRules[i]?.id;
    if (id) return this.store.editSettings((api) => api.removeMeetingRule(id));
    return;
  }

  /** Reads Google Calendar from calendar.google.com with the Google profiles' sign-in. */
  protected async readWeb() {
    await this.store.run('fetchGoogleCalendar');
    this.status.reload();
  }

  protected async google(connect: boolean) {
    try {
      await (connect ? this.api.connectGoogle() : this.api.disconnectGoogle());
    } catch (e) {
      this.store.report(e);
    }
    this.status.reload();
  }
}
