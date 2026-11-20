/** "October 2026" / "2026. október". Months are "YYYY-MM" strings, the same keys the Go backend uses. */
export function monthTitle(month: string, lang: string): string {
  if (!month) return '';
  const [y, m] = month.split('-').map(Number);
  return new Intl.DateTimeFormat(lang, { year: 'numeric', month: 'long' }).format(new Date(y, m - 1, 1));
}
