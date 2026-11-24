import { type ReactNode, useMemo, useState } from "react";
import { computeSummary } from "@/lib/summary";
import { parseLocalDate } from "@/lib/localDate";
import { useToday } from "@/lib/useToday";
import type { Participant } from "@/lib/types";
import Button from "@/components/Button/Button";
import PreferencePill, {
  type PreferencePillVoter,
} from "@/components/PreferencePill/PreferencePill";
import NotesPill, { type NotesPillNote } from "@/components/NotesPill/NotesPill";
import { formatShortDay } from "@/lib/dates";
import { useDateLocale } from "@/lib/dateLocale";
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

interface FinaliseButtonProps {
  date: string;
  onFinalise: (date: string) => Promise<void> | void;
}

function FinaliseButton({ date, onFinalise }: FinaliseButtonProps) {
  const locale = useDateLocale();
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
        aria-label={`Finalise ${formatShortDay(date, locale)}`}
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

interface SummaryRowProps {
  label: ReactNode;
  meta: ReactNode;
  action: ReactNode;
  accent?: boolean;
}

// Two-column grid: date on line 1, pills on line 2 (only when there are any),
// the action centred on the right across both. Nothing in the row can push it
// wider than the card.
function SummaryRow({ label, meta, action, accent }: SummaryRowProps) {
  const classes = [accent ? styles.dateItemAccent : styles.dateItem];
  if (!meta) classes.push(styles.oneLine);
  return (
    <li className={classes.join(" ")}>
      <span className={styles.date}>{label}</span>
      {meta ? (
        <span className={styles.meta} data-testid="summary-row-meta">
          {meta}
        </span>
      ) : null}
      {action ? <span className={styles.action}>{action}</span> : null}
    </li>
  );
}

// The best date gets one full-width primary action; the date is in the label.
function PrimaryFinalise({ date, onFinalise }: FinaliseButtonProps) {
  const locale = useDateLocale();
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
        Finalise {formatShortDay(date, locale)}
      </Button>
    );
  }

  return (
    <div className={styles.primaryConfirm}>
      <Button variant="primary" size="lg" fullWidth loading={submitting} onClick={handleCommit}>
        {submitting ? "Confirming…" : `Confirm ${formatShortDay(date, locale)}?`}
      </Button>
      <Button
        variant="secondary"
        size="lg"
        disabled={submitting}
        onClick={() => setConfirming(false)}
      >
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
  const locale = useDateLocale();
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
  const finaliseFor = (date: string) =>
    showFinalise && date !== topDate ? (
      <FinaliseButton date={date} onFinalise={onFinalise!} />
    ) : null;

  return (
    <div className={styles.section}>
      <h3 className={styles.heading}>Summary</h3>

      <h4 className={styles.subHeading}>Days everyone is free</h4>
      {summary.everyoneFree.length > 0 ? (
        <ul className={styles.dateList}>
          {summary.everyoneFree.map((d) => {
            const voters = votersFor(d.date);
            const notes = notesFor(d.date);
            return (
              <SummaryRow
                key={d.date}
                accent
                label={formatShortDay(d.date, locale)}
                action={finaliseFor(d.date)}
                meta={
                  voters.length > 0 || notes.length > 0 ? (
                    <>
                      <PreferencePill voters={voters} calendarId={calendarId} date={d.date} />
                      <NotesPill
                        notes={notes}
                        calendarId={calendarId}
                        date={d.date}
                        onOpenDay={onOpenDay}
                      />
                    </>
                  ) : null
                }
              />
            );
          })}
        </ul>
      ) : (
        <p className={styles.emptyState}>No days yet</p>
      )}

      {summary.mostFree.length > 0 && (
        <>
          <h4 className={styles.subHeading}>Days most people are free</h4>
          <ul className={styles.dateList}>
            {summary.mostFree.map((d) => {
              const voters = votersFor(d.date);
              return (
                <SummaryRow
                  key={d.date}
                  label={
                    <>
                      {formatShortDay(d.date, locale)}{" "}
                      <span className={styles.countLabel}>
                        — {d.freeCount}/{d.totalCount} free
                      </span>
                    </>
                  }
                  action={finaliseFor(d.date)}
                  meta={
                    voters.length > 0 ? (
                      <PreferencePill voters={voters} calendarId={calendarId} date={d.date} />
                    ) : null
                  }
                />
              );
            })}
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
