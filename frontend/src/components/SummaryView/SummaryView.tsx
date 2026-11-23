import { useMemo, useState } from "react";
import { computeSummary } from "@/lib/summary";
import { parseLocalDate } from "@/lib/localDate";
import { useToday } from "@/lib/useToday";
import type { Participant } from "@/lib/types";
import Button from "@/components/Button/Button";
import PreferencePill, {
  type PreferencePillVoter,
} from "@/components/PreferencePill/PreferencePill";
import NotesPill, { type NotesPillNote } from "@/components/NotesPill/NotesPill";
import styles from "./SummaryView.module.css";

interface SummaryViewProps {
  participants: Participant[];
  freeByDate: Map<string, Set<string>>;
  preferredByDate?: Map<string, Set<string>>;
  notesByDateAndAuthor?: Map<string, Map<string, string>>;
  calendarId?: string;
  canFinalise?: boolean;
  onFinalise?: (date: string) => Promise<void> | void;
  onOpenDay?: (date: string) => void;
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
        {submitting ? "Confirming…" : "Confirm?"}
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

// The best date gets one full-width primary action; the date is in the label.
function PrimaryFinalise({ date, onFinalise }: FinaliseButtonProps) {
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
      <Button variant="primary" size="lg" fullWidth onClick={() => setConfirming(true)}>
        Finalise {formatDate(date)}
      </Button>
    );
  }

  return (
    <div className={styles.primaryConfirm}>
      <Button variant="primary" size="lg" fullWidth loading={submitting} onClick={handleCommit}>
        {submitting ? "Confirming…" : `Confirm ${formatDate(date)}?`}
      </Button>
      <Button variant="secondary" size="lg" disabled={submitting} onClick={() => setConfirming(false)}>
        Cancel
      </Button>
    </div>
  );
}

// The date the owner is most likely to finalise: first everyone-free day,
// else the first most-free day.
export function useTopDate(
  participants: Participant[],
  freeByDate: Map<string, Set<string>>,
): string | null {
  const today = useToday();
  return useMemo(() => {
    const summary = computeSummary(participants, freeByDate, parseLocalDate(today));
    return summary.everyoneFree[0]?.date ?? summary.mostFree[0]?.date ?? null;
  }, [participants, freeByDate, today]);
}

export default function SummaryView({
  participants,
  freeByDate,
  preferredByDate,
  notesByDateAndAuthor,
  calendarId,
  canFinalise = false,
  onFinalise,
  onOpenDay,
}: SummaryViewProps) {
  const today = useToday();
  const summary = useMemo(
    () => computeSummary(participants, freeByDate, parseLocalDate(today)),
    [participants, freeByDate, today],
  );

  const participantById = useMemo(() => {
    const map = new Map<string, Participant>();
    for (const p of participants) map.set(p.id, p);
    return map;
  }, [participants]);

  const notesFor = (date: string): NotesPillNote[] => {
    const byAuthor = notesByDateAndAuthor?.get(date);
    if (!byAuthor || byAuthor.size === 0) return [];
    const result: NotesPillNote[] = [];
    for (const [authorId, text] of byAuthor) {
      if (!text) continue;
      const p = participantById.get(authorId);
      if (!p) continue;
      result.push({ id: p.id, name: p.name, colour: p.colour, text });
    }
    return result;
  };

  const votersFor = (date: string): PreferencePillVoter[] => {
    const ids = preferredByDate?.get(date);
    if (!ids || ids.size === 0) return [];
    const voters: PreferencePillVoter[] = [];
    for (const id of ids) {
      const p = participantById.get(id);
      if (!p) continue;
      voters.push({ id: p.id, name: p.name, colour: p.colour });
    }
    return voters;
  };

  if (participants.length < 2) return null;

  const showFinalise = canFinalise && !!onFinalise;
  const topDate = summary.everyoneFree[0]?.date ?? summary.mostFree[0]?.date ?? null;

  return (
    <div className={styles.section}>
      <h3 className={styles.heading}>Summary</h3>

      <h4 className={styles.subHeading}>Days everyone is free</h4>
      {summary.everyoneFree.length > 0 ? (
        <ul className={styles.dateList}>
          {summary.everyoneFree.map((d) => (
            <li key={d.date} className={styles.dateItemAccent}>
              <span>{formatDate(d.date)}</span>
              <span className={styles.rowEnd}>
                {showFinalise && d.date !== topDate ? (
                  <FinaliseButton date={d.date} onFinalise={onFinalise!} />
                ) : null}
                <PreferencePill voters={votersFor(d.date)} calendarId={calendarId} date={d.date} />
                <NotesPill
                  notes={notesFor(d.date)}
                  calendarId={calendarId}
                  date={d.date}
                  onOpenDay={onOpenDay}
                />
              </span>
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
                <span className={styles.rowEnd}>
                  {showFinalise && d.date !== topDate ? (
                  <FinaliseButton date={d.date} onFinalise={onFinalise!} />
                ) : null}
                  <PreferencePill
                    voters={votersFor(d.date)}
                    calendarId={calendarId}
                    date={d.date}
                  />
                </span>
              </li>
            ))}
          </ul>
        </>
      )}

      {showFinalise && topDate ? (
        <div className={styles.primaryAction}>
          <PrimaryFinalise date={topDate} onFinalise={onFinalise!} />
        </div>
      ) : null}
    </div>
  );
}
