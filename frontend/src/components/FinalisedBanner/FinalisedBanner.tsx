"use client";

import { useState } from "react";
import styles from "./FinalisedBanner.module.css";

interface FinalisedBannerProps {
  date: string;
  emoji?: string;
  canUnfinalise?: boolean;
  onUnfinalise?: () => Promise<void> | void;
}

const fullDateFormatter = new Intl.DateTimeFormat("en-GB", {
  weekday: "long",
  day: "numeric",
  month: "long",
});

function formatFullDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-").map(Number);
  return fullDateFormatter.format(new Date(y, m - 1, d));
}

export default function FinalisedBanner({
  date,
  emoji = "📅",
  canUnfinalise = false,
  onUnfinalise,
}: FinalisedBannerProps) {
  const [confirming, setConfirming] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  async function handleCommit() {
    if (!onUnfinalise) return;
    setSubmitting(true);
    try {
      await onUnfinalise();
    } finally {
      setSubmitting(false);
      setConfirming(false);
    }
  }

  return (
    <div className={styles.banner} role="status">
      <div className={styles.contents}>
        <span className={styles.emoji} aria-hidden="true">
          {emoji}
        </span>
        <div>
          <p className={styles.label}>Confirmed</p>
          <p className={styles.date}>{formatFullDate(date)}</p>
        </div>
      </div>
      {canUnfinalise && onUnfinalise ? (
        <div className={styles.actions}>
          {confirming ? (
            <>
              <button
                type="button"
                className={styles.confirmButton}
                onClick={handleCommit}
                disabled={submitting}
              >
                {submitting ? "Reopening…" : "Reopen calendar?"}
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
            </>
          ) : (
            <button
              type="button"
              className={styles.unfinaliseButton}
              onClick={() => setConfirming(true)}
            >
              Unfinalise
            </button>
          )}
        </div>
      ) : null}
    </div>
  );
}
