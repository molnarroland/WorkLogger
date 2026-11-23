import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { NgIcon, provideIcons } from '@ng-icons/core';
import { phosphorFolderOpen, phosphorFolderPlus, phosphorTrash } from '@ng-icons/phosphor-icons/regular';
import { SnSettingsImports } from '@spartan-native/desktop-ui';
import { NATIVE_HOST } from '@spartan-native/theme';
import { HlmButtonImports } from '@spartan-ng/helm/button';
import { API } from '../../api/api';
import { TPipe } from '../../i18n/t.pipe';
import { ShellStore } from '../../state/shell.store';
import { ChipListComponent } from '../../ui/chip-list/chip-list.component';

/** Repositories → Setup: the folders scanned for git repositories, your commit emails and the skipped folders. */
@Component({
  selector: 'wl-repo-setup',
  imports: [ChipListComponent, HlmButtonImports, NgIcon, SnSettingsImports, TPipe],
  providers: [provideIcons({ phosphorFolderOpen, phosphorFolderPlus, phosphorTrash })],
  templateUrl: './repo-setup.component.html',
  styleUrl: './repositories-page.component.css',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class RepoSetupComponent {
  protected readonly store = inject(ShellStore);
  private readonly api = inject(API);
  private readonly host = inject(NATIVE_HOST);

  /** Adds a workspace folder: the OS folder picker through the host where it gives a path (Wails), the
   *  backend's otherwise. Then scans, so its repositories show straight away. */
  protected async choose() {
    let path: string | null = null;
    if (this.host.dialogs) {
      const picked = await this.host.dialogs.pickFolder({
        directory: this.store.settings()?.workspaceRoots[0] || undefined,
      });
      if (!picked) return;
      path = picked.path ?? null;
    }
    path ??= (await this.store.attempt(() => this.api.chooseFolder())) ?? null;
    if (!path) return;
    await this.store.editSettings((api) => api.addWorkspaceRoot(path));
    await this.store.attempt(() => this.api.scanWorkspace());
  }

  /** Off the list only; its repositories stay until removed (the folder on disk is not touched). */
  protected removeRoot(path: string) {
    return this.store.editSettings((api) => api.removeWorkspaceRoot(path));
  }
}
