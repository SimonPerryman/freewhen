const MINUTE = 60;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const WEEK = 7 * DAY;

export function formatRelative(iso: string, now: Date = new Date()): string {
  const then = new Date(iso);
  const diffSec = Math.max(0, Math.floor((now.getTime() - then.getTime()) / 1000));

  if (diffSec < MINUTE) return "just now";
  if (diffSec < HOUR) return `${Math.floor(diffSec / MINUTE)}m ago`;
  if (diffSec < DAY) return `${Math.floor(diffSec / HOUR)}h ago`;
  if (diffSec < WEEK) return `${Math.floor(diffSec / DAY)}d ago`;
  if (diffSec < 4 * WEEK) return `${Math.floor(diffSec / WEEK)}w ago`;

  return then.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}

export function formatFinalisedDate(date: string): string {
  const [yearStr, monthStr, dayStr] = date.split("-");
  const year = Number(yearStr);
  const month = Number(monthStr);
  const day = Number(dayStr);
  if (!year || !month || !day) return date;
  const d = new Date(Date.UTC(year, month - 1, day));
  return d.toLocaleDateString(undefined, {
    weekday: "short",
    day: "numeric",
    month: "short",
    timeZone: "UTC",
  });
}
