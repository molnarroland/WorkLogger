import type { Router } from '@angular/router';
import type { Focus } from '../api/api';

/**
 * Opens what a focus points at: its page with `?day=` (that day only) and `?focus=` (that item, highlighted or
 * selected). The pages read both as inputs (withComponentInputBinding).
 */
export function openFocus(router: Router, f: Focus) {
  return router.navigate([f.pane], { queryParams: { day: f.day || null, focus: f.id || null } });
}

/** Back to the whole page: drops `?day=` and `?focus=`. */
export function clearFocus(router: Router) {
  return router.navigate([], { queryParams: { day: null, focus: null }, queryParamsHandling: 'merge' });
}

/** The button's label: "Show in Meetings" (i18n key, with the page's name as `page`). */
export const FOCUS_LABEL = 'focus.showIn';
