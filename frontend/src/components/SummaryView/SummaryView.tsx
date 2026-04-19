import { useMemo, useState } from "react";
import { computeSummary } from "@/lib/summary";
import type { Participant } from "@/lib/types";
import styles from "./SummaryView.module.css";

interface SummaryViewProps {
  participants: Participant[];
  freeByDate: Map<string, Set<string>>;
  canFinalise?: boolean;
  onFinalise?: (date: string) => Promise<void> | void;
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

interface FinaliseButtonProps {
  date: string;
  onFinalise: (date: string) => Promise<void> | void;
}

function FinaliseButton({ date, onFinalise }: FinaliseButtonProps) {
  const [confirming, setConfirming] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  async function handleCommit() {
    setSubmitting(true);
    try {
      await onFinalise(date);
    } finally {
      setSubmitting(false);
      setConfirming(false);
    }
  }

  if (!confirming) {
    return (
      <button
        type="button"
        className={styles.finaliseButton}
        onClick={() => setConfirming(true)}
        aria-label={`Finalise ${formatDate(date)}`}
      >
        Finalise
      </button>
    );
  }

  return (
    <span className={styles.confirmGroup}>
      <button
        type="button"
        className={styles.confirmButton}
        onClick={handleCommit}
        disabled={submitting}
      >
        {submitting ? "Confirming…" : `Confirm ${formatDate(date)}?`}
      </button>
      <button
        type="button"
        className={styles.cancelButton}
        onClick={() => setConfirming(false)}
        disabled={submitting}
        aria-label="Cancel"
      >
        ✕
      </button>
    </span>
  );
}

export default function SummaryView({
  participants,
  freeByDate,
  canFinalise = false,
  onFinalise,
}: SummaryViewProps) {
  const summary = useMemo(
    () => computeSummary(participants, freeByDate, new Date()),
    [participants, freeByDate],
  );

  if (participants.length < 2) return null;

  const showFinalise = canFinalise && !!onFinalise;

  return (
    <div className={styles.section}>
      <h3 className={styles.heading}>Summary</h3>

      <h4 className={styles.subHeading}>Days everyone is free</h4>
      {summary.everyoneFree.length > 0 ? (
        <ul className={styles.dateList}>
          {summary.everyoneFree.map((d) => (
            <li key={d.date} className={styles.dateItemAccent}>
              <span>{formatDate(d.date)}</span>
              {showFinalise ? <FinaliseButton date={d.date} onFinalise={onFinalise!} /> : null}
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
                <span>
                  {formatDate(d.date)}{" "}
                  <span className={styles.countLabel}>
                    — {d.freeCount}/{d.totalCount} free
                  </span>
                </span>
                {showFinalise ? <FinaliseButton date={d.date} onFinalise={onFinalise!} /> : null}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
