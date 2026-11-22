import { ChangeDetectionStrategy, Component, computed, inject, input, model, resource, signal } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { lucideChevronDown, lucideChevronRight, lucideFileDiff, lucideGitCommitHorizontal } from '@ng-icons/lucide';
import { SnModalSheet, SnStatusPage } from '@spartan-native/desktop-ui';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { HlmSpinnerImports } from '@spartan-ng/helm/spinner';
import { API, type CommitDiffDto, type CommitDto, type CommitFileDto } from '../../api/api';
import { I18n, TPipe } from '../../i18n/t.pipe';
import { CopyButtonComponent } from '../../ui/copy-button/copy-button.component';

/** A diff line's kind, for its colour. */
type LineKind = 'add' | 'del' | 'hunk' | 'meta' | 'ctx';

interface Open {
  files?: CommitFileDto[];
  error?: string;
  /** The file whose patch shows ('' for the whole commit), and the patch once read. */
  path?: string;
  diff?: CommitDiffDto;
}

/**
 * The commits a git entry was made from: each one's message, the files it changed and, for a file, its patch.
 * Reads the commits from the store when it opens, and the files and patches from the repository on demand.
 */
@Component({
  selector: 'wl-commit-sheet',
  imports: [CopyButtonComponent, HlmButtonImports, HlmSpinnerImports, NgIcon, SnModalSheet, SnStatusPage, TPipe],
  providers: [provideIcons({ lucideChevronDown, lucideChevronRight, lucideFileDiff, lucideGitCommitHorizontal })],
  templateUrl: './commit-sheet.component.html',
  styleUrl: './commit-sheet.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CommitSheetComponent {
  private readonly api = inject(API);
  private readonly i18n = inject(I18n);

  readonly open = model(false);
  /** The git entry's id. */
  readonly worklogId = input.required<number>();
  /** The sheet's title (the entry's ticket or description). */
  readonly title = input('');

  protected readonly commits = resource({
    params: () => (this.open() ? { id: this.worklogId() } : undefined),
    loader: ({ params }) => this.api.worklogCommits(params.id),
  });
  /** Expanded commits by hash. */
  protected readonly expanded = signal<Record<string, Open>>({});
  protected readonly loading = signal<string | null>(null);
  protected readonly heading = computed(() => this.title() || this.i18n.t('commits.title'));

  protected short(hash: string) {
    return hash.slice(0, 8);
  }

  protected when(at: string) {
    return new Date(at).toLocaleString(this.i18n.lang(), {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  protected branch(ref: string) {
    return ref.replace(/^refs\/(heads|remotes)\//, '');
  }

  protected async toggle(c: CommitDto) {
    const all = { ...this.expanded() };
    if (all[c.hash]) {
      delete all[c.hash];
      this.expanded.set(all);
      return;
    }
    this.expanded.set({ ...all, [c.hash]: {} });
    await this.load(c.hash, async () => ({ files: await this.api.commitFiles(c.repoId, c.hash) }));
  }

  /** Shows a file's patch ('' for the whole commit); again hides it. */
  protected async showDiff(c: CommitDto, path: string) {
    const cur = this.expanded()[c.hash];
    if (cur?.path === path) {
      this.patch(c.hash, { path: undefined, diff: undefined });
      return;
    }
    this.patch(c.hash, { path, diff: undefined, error: undefined });
    await this.load(c.hash, async () => ({ path, diff: await this.api.commitDiff(c.repoId, c.hash, path) }));
  }

  protected lines(text: string): { kind: LineKind; text: string }[] {
    return text.split('\n').map((line) => ({ kind: lineKind(line), text: line }));
  }

  private async load(hash: string, op: () => Promise<Open>) {
    this.loading.set(hash);
    try {
      this.patch(hash, await op());
    } catch (e) {
      this.patch(hash, { error: this.i18n.error(e) });
    } finally {
      this.loading.set(null);
    }
  }

  private patch(hash: string, p: Open) {
    const all = this.expanded();
    if (!all[hash]) return; // closed meanwhile
    this.expanded.set({ ...all, [hash]: { ...all[hash], ...p } });
  }
}

function lineKind(line: string): LineKind {
  if (line.startsWith('+++') || line.startsWith('---') || line.startsWith('diff ') || line.startsWith('index '))
    return 'meta';
  if (line.startsWith('@@')) return 'hunk';
  if (line.startsWith('+')) return 'add';
  if (line.startsWith('-')) return 'del';
  return 'ctx';
}
