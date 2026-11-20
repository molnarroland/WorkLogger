import { Pipe, PipeTransform } from '@angular/core';
import { formatDuration } from './worklogs';

/** `{{ seconds | duration }}` → "1h 30m". Pair with the `tabular` class. */
@Pipe({ name: 'duration' })
export class DurationPipe implements PipeTransform {
  transform(seconds: number | null | undefined): string {
    return formatDuration(seconds ?? 0);
  }
}
