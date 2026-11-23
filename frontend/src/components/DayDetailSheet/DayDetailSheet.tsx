"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { Availability, Participant } from "@/lib/types";
import { MAX_NOTE_LENGTH, codePointCount } from "@/lib/limits";
import CharCounter, { isAtLimit } from "@/components/CharCounter/CharCounter";
import Button from "@/components/Button/Button";
import Sheet from "@/components/Sheet/Sheet";
import styles from "./DayDetailSheet.module.css";

interface DayDetailSheetProps {
  date: string;
  participants: Participant[];
  availability: Availability[];
  currentParticipantId: string | null;
  // Current participant's own note for this date, if any. Sheet echoes this
  // back as the textarea's initial value so editing is a consistent surface.
  currentNote: string;
  isSelected: boolean;
  finalised: boolean;
  // Current participant's own vote for this date. Toggled via onToggleVote.
  isPreferred?: boolean;
  // Set of participant IDs (including current) that have voted for this date;
  // drives the ★ indicator next to each name in the "who's free" list.
  preferredParticipantIds?: Set<string>;
  onSaveNote: (text: string) => void;
  onToggleVote?: (preferred: boolean) => void;
  onRemoveAvailability: () => void;
  onClose: () => void;
}

const PREFER_HINT_KEY = "freewhen:prefer-hint-seen";

function readPreferHintSeen(): boolean {
  try {
    return window.localStorage.getItem(PREFER_HINT_KEY) === "1";
  } catch {
    return false;
  }
}

function writePreferHintSeen() {
  try {
    window.localStorage.setItem(PREFER_HINT_KEY, "1");
  } catch {
    // Storage blocked: the hint just keeps showing.
  }
}

const dateFormatter = new Intl.DateTimeFormat("en-GB", {
  weekday: "long",
  day: "numeric",
  month: "long",
});

function formatFullDate(dateStr: string): string {
  const [y, m, d] = dateStr.split("-").map(Number);
  return dateFormatter.format(new Date(y, m - 1, d));
}

// Strip CR/LF so the cell preview can't wrap and the server doesn't reject
// on submit. Matches the "newlines rejected entirely" rule.
function stripNewlines(s: string): string {
  return s.replace(/[\r\n]+/g, " ");
}

// Cap a string to MAX_NOTE_LENGTH code points. Slicing by UTF-16 code units
// (String.prototype.slice) would split surrogate pairs; iterating with for…of
// walks code points so emoji stay intact.
function truncateToCodePoints(s: string, max: number): string {
  let count = 0;
  let cut = 0;
  for (const ch of s) {
    if (count >= max) break;
    cut += ch.length;
    count += 1;
  }
  return s.slice(0, cut);
}

export default function DayDetailSheet({
  date,
  participants,
  availability,
  currentParticipantId,
  currentNote,
  isSelected,
  finalised,
  isPreferred = false,
  preferredParticipantIds,
  onSaveNote,
  onToggleVote,
  onRemoveAvailability,
  onClose,
}: DayDetailSheetProps) {
  const [draft, setDraft] = useState(currentNote);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  // Starts true so server and first client render match; the effect below
  // reveals the hint for people who haven't preferred a date yet.
  const [preferHintSeen, setPreferHintSeen] = useState(true);

  useEffect(() => {
    setPreferHintSeen(readPreferHintSeen());
  }, []);

  useEffect(() => {
    setDraft(currentNote);
  }, [currentNote, date]);

  // After the trap focuses the textarea, push the caret to the end so
  // editing an existing note doesn't force the user to Ctrl-End first.
  useEffect(() => {
    if (finalised) return;
    const ta = textareaRef.current;
    if (!ta) return;
    const len = ta.value.length;
    ta.setSelectionRange(len, len);
  }, [finalised]);

  // Build the "who's free" list with notes. We overlay current-participant
  // state from the parent so the list is consistent with their unsaved
  // toggle/note edits.
  const rows = useMemo(() => {
    const byId = new Map(participants.map((p) => [p.id, p]));
    const freeOnDay = availability.filter((a) => a.date === date);
    const result: { participant: Participant; note: string | null; isMe: boolean }[] = [];

    for (const a of freeOnDay) {
      const p = byId.get(a.participantId);
      if (!p) continue;
      const isMe = a.participantId === currentParticipantId;
      // For the current participant, prefer the local draft-committed note
      // (currentNote) over the backend's; for others, trust the row's note.
      const note = isMe ? currentNote || null : (a.note ?? null);
      result.push({ participant: p, note, isMe });
    }

    // If the current user has selected the day but no backend availability row
    // has surfaced yet (optimistic state), include them.
    if (
      currentParticipantId &&
      isSelected &&
      !result.some((r) => r.participant.id === currentParticipantId)
    ) {
      const p = byId.get(currentParticipantId);
      if (p) {
        result.push({
          participant: p,
          note: currentNote || null,
          isMe: true,
        });
      }
    }

    // Stable sort: current user first, then by colour-sort-key proxy (name).
    result.sort((a, b) => {
      if (a.isMe && !b.isMe) return -1;
      if (!a.isMe && b.isMe) return 1;
      return a.participant.name.localeCompare(b.participant.name);
    });
    return result;
  }, [participants, availability, date, currentParticipantId, currentNote, isSelected]);

  const draftCount = codePointCount(draft);
  const tooLong = draftCount > MAX_NOTE_LENGTH;
  const atLimit = isAtLimit(draftCount, MAX_NOTE_LENGTH);
  const trimmed = draft.trim();
  const isWhitespaceOnly = draft.length > 0 && trimmed.length === 0;
  const canSave = !tooLong && !isWhitespaceOnly;
  const noteChanged = draft.trim() !== currentNote.trim();

  function handleTextChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    // Strip newlines on entry so the counter and the eventual save match.
    const cleaned = stripNewlines(e.target.value);
    const capped =
      codePointCount(cleaned) > MAX_NOTE_LENGTH
        ? truncateToCodePoints(cleaned, MAX_NOTE_LENGTH)
        : cleaned;
    setDraft(capped);
  }

  function handleSave() {
    if (!canSave) return;
    onSaveNote(trimmed);
    onClose();
  }

  const editable = !finalised && isSelected && currentParticipantId !== null;

  return (
    <Sheet
      title={<span className={styles.title}>{formatFullDate(date)}</span>}
      onClose={onClose}
      closeLabel="Close day details"
      // Without an editable note the close button (first focusable) gets focus.
      initialFocus={finalised ? undefined : textareaRef}
    >
      <section className={styles.section} aria-labelledby="day-sheet-free-label">
        <h3 id="day-sheet-free-label" className={styles.sectionLabel}>
          {rows.length > 0
            ? rows.length === 1
              ? "1 person free"
              : `${rows.length} people free`
            : "No one's free"}
        </h3>
        {rows.length > 0 ? (
          <ul className={styles.whoList}>
            {rows.map(({ participant, note, isMe }) => (
              <li key={participant.id} className={styles.whoItem}>
                <span
                  className={styles.swatch}
                  style={{ backgroundColor: participant.colour }}
                  aria-hidden="true"
                />
                <div className={styles.whoBody}>
                  <span className={styles.whoName}>
                    {participant.name}
                    {isMe ? <span className={styles.meTag}>you</span> : null}
                    {preferredParticipantIds?.has(participant.id) ? (
                      <span
                        className={styles.preferStar}
                        aria-label={`${participant.name} prefers this date`}
                      >
                        ★
                      </span>
                    ) : null}
                  </span>
                  {note ? <span className={styles.whoNote}>{note}</span> : null}
                </div>
              </li>
            ))}
          </ul>
        ) : null}
      </section>

      {editable && onToggleVote ? (
        <section className={styles.section}>
          <label className={styles.preferLabel}>
            <input
              type="checkbox"
              className={styles.preferCheckbox}
              checked={isPreferred}
              onChange={(e) => {
                if (e.target.checked) {
                  writePreferHintSeen();
                  setPreferHintSeen(true);
                }
                onToggleVote(e.target.checked);
              }}
            />
            <span className={styles.preferText}>
              <span className={styles.preferTitle}>Prefer this date</span>
              {preferHintSeen ? null : (
                <span className={styles.preferHint}>
                  Mark your top picks — helps the group pick a date everyone actually wants.
                </span>
              )}
            </span>
          </label>
        </section>
      ) : null}

      {editable ? (
        <section className={styles.section}>
          <label className={styles.editorLabel} htmlFor="day-sheet-note">
            Note (optional)
          </label>
          <textarea
            ref={textareaRef}
            id="day-sheet-note"
            className={atLimit ? `${styles.textarea} ${styles.textareaAtLimit}` : styles.textarea}
            value={draft}
            onChange={handleTextChange}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                handleSave();
              }
            }}
            placeholder="e.g. only after 6pm"
          />
          <div className={styles.editorFoot}>
            <CharCounter count={draftCount} max={MAX_NOTE_LENGTH} />
            {noteChanged ? (
              <div className={styles.editorActions}>
                <Button variant="secondary" size="sm" onClick={() => setDraft(currentNote)}>
                  Reset
                </Button>
                <Button variant="primary" size="sm" onClick={handleSave} disabled={!canSave}>
                  Save note
                </Button>
              </div>
            ) : null}
          </div>
        </section>
      ) : null}

      {editable ? (
        <button
          type="button"
          className={styles.removeButton}
          onClick={() => {
            onRemoveAvailability();
            onClose();
          }}
        >
          Remove my availability
        </button>
      ) : null}
    </Sheet>
  );
}
