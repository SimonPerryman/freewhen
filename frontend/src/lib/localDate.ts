// Today's date as an ISO `YYYY-MM-DD` string in the browser's local timezone.
// Availability dates are calendar days, not instants, so comparisons against
// "today" must use the local date rather than `toISOString()` (UTC).
export function localToday(now: Date = new Date()): string {
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

// Local midnight of an ISO `YYYY-MM-DD` date.
export function parseLocalDate(iso: string): Date {
  const [y, m, d] = iso.split("-").map(Number);
  return new Date(y, m - 1, d);
}
