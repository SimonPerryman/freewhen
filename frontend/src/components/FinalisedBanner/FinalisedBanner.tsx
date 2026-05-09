"use client";

import { useMemo, useState } from "react";
import type { Availability, Participant } from "@/lib/types";
import ExportButtons from "@/components/ExportButtons/ExportButtons";
import styles from "./FinalisedBanner.module.css";

interface FinalisedBannerProps {
  date: string;
  emoji?: string;
  // When provided, renders export buttons (Google Calendar deep link + .ics
  // download) alongside the unfinalise action.
  calendarId?: string;
  canUnfinalise?: boolean;
  onUnfinalise?: () => Promise<void> | void;
  // Optional — when provided, the banner surfaces per-participant notes for
  // the finalised date inline. The plan's §8 rationale: notes become
  // load-bearing at the exact moment someone needs to decide whether to show
  // up, so we put them next to the confirmed date.
  participants?: Participant[];
  availability?: Availability[];
  // Number of participants who preferred the finalised date. Zero omits the
  // line entirely; the sheet surfaces per-person stars.
  voteCount?: number;
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
  calendarId,
  canUnfinalise = false,
  onUnfinalise,
  participants,
  availability,
  voteCount = 0,
}: FinalisedBannerProps) {
  const [confirming, setConfirming] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const notes = useMemo(() => {
    if (!participants || !availability) return [];
    const byId = new Map(participants.map((p) => [p.id, p]));
    return availability
      .filter((a) => a.date === date && a.note)
      .map((a) => ({ participant: byId.get(a.participantId), note: a.note! }))
      .filter((row): row is { participant: Participant; note: string } => !!row.participant)
      .sort((a, b) => a.participant.name.localeCompare(b.participant.name));
  }, [participants, availability, date]);

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
    <div className={styles.banner} role="status" aria-live="polite">
      <div className={styles.contents}>
        <span className={styles.emoji} aria-hidden="true">
          {emoji}
        </span>
        <div className={styles.details}>
          <p className={styles.label}>Confirmed</p>
          <p className={styles.date}>{formatFullDate(date)}</p>
          {voteCount > 0 ? (
            <p className={styles.voteLine}>
              {voteCount === 1
                ? "1 person preferred this date"
                : `${voteCount} preferred this date`}
            </p>
          ) : null}
          {notes.length > 0 ? (
            <ul className={styles.noteList} aria-label="Notes for this date">
              {notes.map(({ participant, note }) => (
                <li key={participant.id} className={styles.noteItem}>
                  <span
                    className={styles.swatch}
                    style={{ backgroundColor: participant.colour }}
                    aria-hidden="true"
                  />
                  <span className={styles.noteName}>{participant.name}</span>
                  <span className={styles.noteDash} aria-hidden="true">
                    —
                  </span>
                  <span className={styles.noteText}>{note}</span>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      </div>
      {calendarId ? (
        <div className={styles.exportRow}>
          <ExportButtons calendarId={calendarId} />
        </div>
      ) : null}
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
