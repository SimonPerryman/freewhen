import type { Participant } from "./types";

export interface DateSummary {
  date: string;
  freeCount: number;
  totalCount: number;
}

export interface SummaryResult {
  everyoneFree: DateSummary[];
  mostFree: DateSummary[];
}

export function computeSummary(
  participants: Participant[],
  freeByDate: Map<string, Set<string>>,
  today: Date,
): SummaryResult {
  const totalCount = participants.length;
  const todayStr = formatDateKey(today);

  const everyoneFree: DateSummary[] = [];
  const mostFree: DateSummary[] = [];

  for (const [date, freeSet] of freeByDate) {
    if (date < todayStr) continue;

    const freeCount = freeSet.size;
    if (freeCount === 0) continue;

    const entry: DateSummary = { date, freeCount, totalCount };

    if (freeCount === totalCount) {
      everyoneFree.push(entry);
    } else if (freeCount >= 2) {
      mostFree.push(entry);
    }
  }

  everyoneFree.sort((a, b) => a.date.localeCompare(b.date));
  mostFree.sort((a, b) => b.freeCount - a.freeCount || a.date.localeCompare(b.date));

  return {
    everyoneFree,
    mostFree: mostFree.slice(0, 5),
  };
}

function formatDateKey(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}
