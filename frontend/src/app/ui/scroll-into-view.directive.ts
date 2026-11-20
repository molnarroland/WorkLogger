import { Directive, ElementRef, afterRenderEffect, inject, input } from '@angular/core';

/** Scrolls the element into view when the input turns true: the item a link from another page points at. */
@Directive({ selector: '[wlScrollIntoView]' })
export class ScrollIntoViewDirective {
  readonly wlScrollIntoView = input(false);
  private readonly el: HTMLElement = inject(ElementRef<HTMLElement>).nativeElement;

  constructor() {
    afterRenderEffect(() => {
      if (this.wlScrollIntoView()) this.el.scrollIntoView?.({ block: 'center' });
    });
  }
}
