"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { Availability, Participant } from "@/lib/types";
import { MAX_NOTE_LENGTH, codePointCount } from "@/lib/limits";
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
  onSaveNote: (text: string) => void;
  onRemoveAvailability: () => void;
  onClose: () => void;
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
  onSaveNote,
  onRemoveAvailability,
  onClose,
}: DayDetailSheetProps) {
  const [draft, setDraft] = useState(currentNote);
  const backdropRef = useRef<HTMLDivElement | null>(null);
  const sheetRef = useRef<HTMLDivElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    setDraft(currentNote);
  }, [currentNote, date]);

  // Esc-to-close + minimal focus trap. When the editor is shown, focus the
  // textarea so the user can type immediately; otherwise fall back to the
  // close button so keyboard users have an obvious escape.
  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null;
    if (!finalised && textareaRef.current) {
      textareaRef.current.focus();
      // Put the caret at the end so editing an existing note doesn't force
      // the user to Ctrl-End before appending.
      const len = textareaRef.current.value.length;
      textareaRef.current.setSelectionRange(len, len);
    } else {
      closeButtonRef.current?.focus();
    }

    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.stopPropagation();
        onClose();
      }
      if (e.key === "Tab") {
        // Trap focus within the sheet. We scope to focusable elements inside
        // sheetRef and wrap at the boundaries.
        const root = sheetRef.current;
        if (!root) return;
        const focusable = root.querySelectorAll<HTMLElement>(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
        );
        if (focusable.length === 0) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      // Restore focus to the triggering element so keyboard users land back
      // on the cell they tapped.
      prev?.focus?.();
    };
  }, [finalised, onClose]);

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
  const trimmed = draft.trim();
  const isWhitespaceOnly = draft.length > 0 && trimmed.length === 0;
  const canSave = !tooLong && !isWhitespaceOnly;

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

  function handleBackdropClick(e: React.MouseEvent<HTMLDivElement>) {
    if (e.target === backdropRef.current) onClose();
  }

  const editable = !finalised && isSelected && currentParticipantId !== null;

  return (
    <div
      ref={backdropRef}
      className={styles.backdrop}
      onClick={handleBackdropClick}
      role="presentation"
    >
      <div
        ref={sheetRef}
        className={styles.sheet}
        role="dialog"
        aria-modal="true"
        aria-labelledby="day-sheet-title"
      >
        <div className={styles.header}>
          <h2 id="day-sheet-title" className={styles.title}>
            {formatFullDate(date)}
          </h2>
          <button
            ref={closeButtonRef}
            type="button"
            className={styles.closeButton}
            onClick={onClose}
            aria-label="Close day details"
          >
            ✕
          </button>
        </div>

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
                    </span>
                    {note ? <span className={styles.whoNote}>{note}</span> : null}
                  </div>
                </li>
              ))}
            </ul>
          ) : null}
        </section>

        {editable ? (
          <section className={styles.section}>
            <label className={styles.editorLabel} htmlFor="day-sheet-note">
              Your note
              <span className={styles.editorHint}>Optional — 140 characters</span>
            </label>
            <textarea
              ref={textareaRef}
              id="day-sheet-note"
              className={styles.textarea}
              value={draft}
              onChange={handleTextChange}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  handleSave();
                }
              }}
              placeholder="e.g. only after 6pm"
              aria-describedby="day-sheet-note-counter"
            />
            <div className={styles.editorFoot}>
              <span
                id="day-sheet-note-counter"
                className={tooLong ? styles.counterOver : styles.counter}
                aria-live="polite"
              >
                {draftCount}/{MAX_NOTE_LENGTH}
              </span>
              <div className={styles.editorActions}>
                <button
                  type="button"
                  className={styles.cancelButton}
                  onClick={() => setDraft(currentNote)}
                  disabled={draft === currentNote}
                >
                  Reset
                </button>
                <button
                  type="button"
                  className={styles.saveButton}
                  onClick={handleSave}
                  disabled={!canSave || draft.trim() === currentNote.trim()}
                >
                  Save
                </button>
              </div>
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
      </div>
    </div>
  );
}
