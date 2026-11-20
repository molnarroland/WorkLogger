import { DestroyRef, Directive, ElementRef, afterNextRender, inject, signal } from '@angular/core';

/**
 * On the header line: how wide its centred title may be without running into the month on the left
 * (`.month-nav`) or the first command on the right. The title is centred on the whole line, so whenever the line or
 * anything on it changes size (a page's commands come and go, the month's name, the language), the room is
 * measured again. `room() < 48` means too narrow to show (the sidebar still names the page).
 *
 * `<sn-command-bar wlTitleFit #fit="titleFit">…<h1 [style.max-inline-size.px]="fit.room()">`
 */
@Directive({ selector: '[wlTitleFit]', exportAs: 'titleFit' })
export class TitleFitDirective {
  /** Width in px. */
  readonly room = signal(400);
  private readonly el: HTMLElement = inject(ElementRef<HTMLElement>).nativeElement;

  constructor() {
    const destroyRef = inject(DestroyRef);
    afterNextRender(() => {
      if (typeof ResizeObserver === 'undefined') return;
      const resize = new ResizeObserver(() => this.measure());
      const watch = () => {
        resize.disconnect();
        resize.observe(this.el);
        for (const child of Array.from(this.el.children)) resize.observe(child);
      };
      const children = new MutationObserver(watch);
      children.observe(this.el, { childList: true });
      watch();
      destroyRef.onDestroy(() => {
        resize.disconnect();
        children.disconnect();
      });
    });
  }

  /** Twice the distance from the line's centre to the nearer neighbour, less a gap on each side. */
  private measure() {
    const left = this.el.querySelector('.month-nav')?.getBoundingClientRect();
    if (!left) return;
    const line = this.el.getBoundingClientRect();
    const center = line.left + line.width / 2;
    const commands = Array.from(this.el.querySelectorAll('[data-slot=sn-command]'))
      .map((b) => b.getBoundingClientRect().left)
      .filter((x) => x > left.right);
    const right = commands.length ? Math.min(...commands) : line.right;
    const gap = 16;
    this.room.set(Math.max(0, Math.floor(2 * Math.min(center - left.right, right - center) - 2 * gap)));
  }
}
