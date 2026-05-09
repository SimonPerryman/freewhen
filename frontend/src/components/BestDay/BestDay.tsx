import { useEffect, useMemo } from "react";
import type { Participant } from "@/lib/types";
import { computeSummary } from "@/lib/summary";
import analytics from "@/lib/analytics";
import styles from "./BestDay.module.css";

interface BestDayProps {
  participants: Participant[];
  freeByDate: Map<string, Set<string>>;
  voteCountsByDate?: Map<string, number>;
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

export default function BestDay({ participants, freeByDate, voteCountsByDate }: BestDayProps) {
  const bestDay = useMemo(() => {
    if (participants.length < 2) return null;

    const summary = computeSummary(participants, freeByDate, new Date());
    const votes = (date: string): number => voteCountsByDate?.get(date) ?? 0;
    const voteSuffix = (date: string): string => {
      const n = votes(date);
      return n > 0 ? ` — ${n} prefer this` : "";
    };

    // Single date where everyone is free.
    if (summary.everyoneFree.length === 1) {
      const d = summary.everyoneFree[0].date;
      return {
        date: d,
        label: `Everyone's free${voteSuffix(d)}`,
        tiebreaker: "availability" as const,
      };
    }

    // Multiple "everyone free" dates — tie-break on votes.
    if (summary.everyoneFree.length > 1) {
      const ranked = [...summary.everyoneFree].sort(
        (a, b) => votes(b.date) - votes(a.date) || a.date.localeCompare(b.date),
      );
      const top = ranked[0];
      const second = ranked[1];
      if (votes(top.date) > votes(second.date)) {
        return {
          date: top.date,
          label: `Everyone's free${voteSuffix(top.date)}`,
          tiebreaker: "votes" as const,
        };
      }
      return null;
    }

    // Most-free winner path.
    if (summary.mostFree.length > 0) {
      const top = summary.mostFree[0];
      if (summary.mostFree.length === 1 || top.freeCount > summary.mostFree[1].freeCount) {
        return {
          date: top.date,
          label: `${top.freeCount} of ${top.totalCount} free${voteSuffix(top.date)}`,
          tiebreaker: "availability" as const,
        };
      }
      // Free-count tie — break on votes.
      const tied = summary.mostFree.filter((d) => d.freeCount === top.freeCount);
      const ranked = [...tied].sort(
        (a, b) => votes(b.date) - votes(a.date) || a.date.localeCompare(b.date),
      );
      const winner = ranked[0];
      const runnerUp = ranked[1];
      if (votes(winner.date) > votes(runnerUp.date)) {
        return {
          date: winner.date,
          label: `${winner.freeCount} of ${winner.totalCount} free${voteSuffix(winner.date)}`,
          tiebreaker: "votes" as const,
        };
      }
    }

    return null;
  }, [participants, freeByDate, voteCountsByDate]);

  useEffect(() => {
    if (bestDay) {
      analytics.capture("best_day_shown", { tiebreaker: bestDay.tiebreaker });
    }
  }, [bestDay]);

  if (!bestDay) return null;

  return (
    <div className={styles.card}>
      <div className={styles.content}>
        <span className={styles.label}>Best so far</span>
        <span className={styles.date}>{formatDate(bestDay.date)}</span>
      </div>
      <span className={styles.subtitle}>{bestDay.label}</span>
    </div>
  );
}
