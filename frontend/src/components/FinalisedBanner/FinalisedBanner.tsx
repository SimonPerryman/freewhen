"use client";

import { useMemo } from "react";
import type { Availability, Participant } from "@/lib/types";
import ExportButtons from "@/components/ExportButtons/ExportButtons";
import { formatLongDay } from "@/lib/dates";
import { useDateLocale } from "@/lib/dateLocale";
import styles from "./FinalisedBanner.module.css";

interface FinalisedBannerProps {
  date: string;
  emoji?: string;
  // When provided, renders export buttons (Google Calendar deep link + .ics
  // download).
  calendarId?: string;
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

export default function FinalisedBanner({
  date,
  emoji = "📅",
  calendarId,
  participants,
  availability,
  voteCount = 0,
}: FinalisedBannerProps) {
  const locale = useDateLocale();
  const notes = useMemo(() => {
    if (!participants || !availability) return [];
    const byId = new Map(participants.map((p) => [p.id, p]));
    return availability
      .filter((a) => a.date === date && a.note)
      .map((a) => ({ participant: byId.get(a.participantId), note: a.note! }))
      .filter((row): row is { participant: Participant; note: string } => !!row.participant)
      .sort((a, b) => a.participant.name.localeCompare(b.participant.name));
  }, [participants, availability, date]);

  return (
    <div className={styles.banner} role="status" aria-live="polite">
      <div className={styles.contents}>
        <span className={styles.emoji} aria-hidden="true">
          {emoji}
        </span>
        <div className={styles.details}>
          <p className={styles.label}>Confirmed</p>
          <p className={styles.date}>{formatLongDay(date, locale)}</p>
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
                  <span className={styles.noteBody}>
                    <span className={styles.noteName}>{participant.name}</span>{" "}
                    <span className={styles.noteDash} aria-hidden="true">
                      —
                    </span>{" "}
                    <span className={styles.noteText}>{note}</span>
                  </span>
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
    </div>
  );
}
