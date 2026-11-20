import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { HlmBadgeImports } from '@spartan-ng/helm/badge';
import { TPipe } from '../../i18n/t.pipe';

/** "signed in" / "signed out" for a site WorkLogger reads from, as a toned badge; nothing until it's been checked. */
@Component({
  selector: 'wl-sign-in-badge',
  imports: [HlmBadgeImports, TPipe],
  templateUrl: './sign-in-badge.component.html',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SignInBadgeComponent {
  readonly state = input<boolean | null>(null);
}
