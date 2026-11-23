import { ChangeDetectionStrategy, Component, computed, inject, input, resource, signal } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { NgIcon, provideIcons } from '@ng-icons/core';
import {
  phosphorGitBranch,
  phosphorHardDrive,
  phosphorArrowsClockwise,
  phosphorMagnifyingGlassMinus,
  phosphorTrash,
} from '@ng-icons/phosphor-icons/regular';
import {
  SnCommandBarImports,
  SnPageCommandsTemplate,
  SnSettingsImports,
  SnStatusPage,
} from '@spartan-native/desktop-ui';
import { HlmAccordionImports } from '@spartan-ng/helm/accordion';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmInputImports } from '@spartan-ng/helm/input';
import { HlmSwitchImports } from '@spartan-ng/helm/switch';
import { HlmTableImports } from '@spartan-ng/helm/table';
import { API, type RepoDto } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import { ShellStore } from '../../state/shell.store';
import { type RuleItem, RuleListComponent } from '../../ui/rule-list/rule-list.component';
import { ListFilterComponent } from '../../ui/list-filter/list-filter.component';

import { ScrollIntoViewDirective } from '../../ui/scroll-into-view.directive';
/** The repositories and their commits, and which issue each repository's time goes to. Folders and commit emails
 *  are on the Setup tab (RepoSetupComponent). */
@Component({
  selector: 'wl-repositories-page',
  imports: [
    ScrollIntoViewDirective,
    RouterLink,
    SnPageCommandsTemplate,
    ListFilterComponent,
    HlmAccordionImports,
    HlmButtonImports,
    HlmInputImports,
    HlmSwitchImports,
    HlmTableImports,
    NgIcon,
    RuleListComponent,
    SnCommandBarImports,
    SnSettingsImports,
    SnStatusPage,
    TPipe,
  ],
  providers: [
    provideIcons({
      phosphorGitBranch,
      phosphorHardDrive,
      phosphorArrowsClockwise,
      phosphorMagnifyingGlassMinus,
      phosphorTrash,
    }),
  ],
  templateUrl: './repositories-page.component.html',
  styleUrl: './repositories-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RepositoriesPageComponent {
  /** `?focus=`: the repository to point at (its id; a link from an entry made from its commits). */
  readonly focus = input<string>();
  /** `?day=`: only the repositories with commits that day (the filter's day picker). */
  readonly day = input<string>();
  protected readonly store = inject(ShellStore);
  private readonly router = inject(Router);
  private readonly api = inject(API);

  protected readonly repos = resource({
    params: () => this.store.month() || undefined,
    loader: ({ params }) => this.api.repos(params),
  });
  /** The search box: matched against the name, the path and the issue and project keys. */
  protected readonly query = signal('');
  /** Off: only the repositories with commits that day (or month); on: every one, to switch on or set up. */
  protected readonly showAll = signal(false);
  /** The repositories with commits in the month, or that day; the focused one always. */
  private readonly withCommits = computed(() => {
    const day = this.day();
    return (this.repos.value() ?? []).filter(
      (r) => '' + r.id === this.focus() || (day ? this.dayCommits(r, day) > 0 : r.commits > 0),
    );
  });
  protected readonly hiddenCount = computed(() => (this.repos.value()?.length ?? 0) - this.withCommits().length);
  protected readonly shown = computed(() => {
    const q = this.query().trim().toLocaleLowerCase();
    return (this.showAll() ? (this.repos.value() ?? []) : this.withCommits()).filter(
      (r) => !q || [r.name, r.path, r.issueKey, r.projectKey].some((v) => !!v && v.toLocaleLowerCase().includes(q)),
    );
  });
  protected readonly projects = resource({ loader: () => this.api.projects() });
  protected readonly projectItems = computed<RuleItem[]>(() =>
    (this.projects.value() ?? []).map((p) => ({ label: p.key, issueKey: p.defaultIssue })),
  );

  protected async scan() {
    await this.store.attempt(() => this.api.scanWorkspace());
    this.repos.reload();
  }

  protected async update(r: RepoDto, patch: Partial<RepoDto>) {
    await this.store.attempt(() => this.api.updateRepo({ ...r, ...patch }));
    this.repos.reload();
  }

  /** What was typed; the backend normalises it (empty: none). */
  protected key(e: Event) {
    return (e.target as HTMLInputElement).value.trim() || null;
  }

  protected async remove(r: RepoDto) {
    await this.store.attempt(() => this.api.removeRepo(r.id));
    this.repos.reload();
  }

  protected async addProject(item: RuleItem) {
    await this.store.attempt(() => this.api.saveProject({ key: item.label, defaultIssue: item.issueKey }));
    this.projects.reload();
  }

  protected async removeProject(i: number) {
    const p = this.projects.value()?.[i];
    if (p) await this.store.attempt(() => this.api.removeProject(p.key));
    this.projects.reload();
  }

  protected dayCommits(r: RepoDto, day: string) {
    return r.days?.[day] ?? 0;
  }

  protected setDay(day: string | null) {
    return this.router.navigate([], { queryParams: { day, focus: null }, queryParamsHandling: 'merge' });
  }

  protected clearFilter() {
    this.query.set('');
    return this.setDay(null);
  }
}
