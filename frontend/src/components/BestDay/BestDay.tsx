import { useMemo } from "react";
import type { Participant } from "@/lib/types";
import { computeSummary } from "@/lib/summary";
import styles from "./BestDay.module.css";

interface BestDayProps {
  participants: Participant[];
  freeByDate: Map<string, Set<string>>;
}

const dateFormatter = new Intl.DateTimeFormat("en-GB", {
  weekday: "long",
  day: "numeric",
  month: "short",
});

function formatDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-").map(Number);
  return dateFormatter.format(new Date(y, m - 1, d));
}

export default function BestDay({ participants, freeByDate }: BestDayProps) {
  const bestDay = useMemo(() => {
    if (participants.length < 2) return null;

    const summary = computeSummary(participants, freeByDate, new Date());

    // If there's exactly one date where everyone is free, that's the best day
    if (summary.everyoneFree.length === 1) {
      return {
        date: summary.everyoneFree[0].date,
        label: "Everyone's free",
      };
    }

    // If nobody is free everywhere but there's a clear "most free" winner
    if (summary.everyoneFree.length === 0 && summary.mostFree.length > 0) {
      const top = summary.mostFree[0];
      // Only show if the top day is clearly ahead of second place
      if (summary.mostFree.length === 1 || top.freeCount > summary.mostFree[1].freeCount) {
        return {
          date: top.date,
          label: `${top.freeCount} of ${top.totalCount} free`,
        };
      }
    }

    return null;
  }, [participants, freeByDate]);

  if (!bestDay) return null;

  return (
    <div className={styles.card}>
      <span className={styles.icon} aria-hidden="true">&#x2713;</span>
      <div className={styles.content}>
        <span className={styles.title}>Best day</span>
        <span className={styles.date}>{formatDate(bestDay.date)}</span>
        <span className={styles.subtitle}>{bestDay.label}</span>
      </div>
    </div>
  );
}
