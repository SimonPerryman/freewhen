import { useMemo } from "react";
import { computeSummary } from "@/lib/summary";
import type { Participant } from "@/lib/types";
import styles from "./SummaryView.module.css";

interface SummaryViewProps {
  participants: Participant[];
  freeByDate: Map<string, Set<string>>;
}

const dateFormatter = new Intl.DateTimeFormat("en-GB", {
  weekday: "short",
  day: "numeric",
  month: "short",
});

function formatDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-").map(Number);
  return dateFormatter.format(new Date(y, m - 1, d));
}

export default function SummaryView({ participants, freeByDate }: SummaryViewProps) {
  const summary = useMemo(
    () => computeSummary(participants, freeByDate, new Date()),
    [participants, freeByDate],
  );

  if (participants.length < 2) return null;

  return (
    <div className={styles.section}>
      <h3 className={styles.heading}>Summary</h3>

      <h4 className={styles.subHeading}>Days everyone is free</h4>
      {summary.everyoneFree.length > 0 ? (
        <ul className={styles.dateList}>
          {summary.everyoneFree.map((d) => (
            <li key={d.date} className={styles.dateItemAccent}>
              {formatDate(d.date)}
            </li>
          ))}
        </ul>
      ) : (
        <p className={styles.emptyState}>No days yet</p>
      )}

      {summary.mostFree.length > 0 && (
        <>
          <h4 className={styles.subHeading}>Days most people are free</h4>
          <ul className={styles.dateList}>
            {summary.mostFree.map((d) => (
              <li key={d.date} className={styles.dateItem}>
                {formatDate(d.date)}{" "}
                <span className={styles.countLabel}>
                  — {d.freeCount}/{d.totalCount} free
                </span>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
