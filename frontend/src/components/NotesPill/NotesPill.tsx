"use client";

import { useCallback, useEffect, useId, useRef, useState } from "react";
import analytics from "@/lib/analytics";
import styles from "./NotesPill.module.css";

export interface NotesPillNote {
  id: string;
  name: string;
  colour: string;
  text: string;
}

interface Props {
  notes: NotesPillNote[];
  calendarId?: string;
  date?: string;
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

export default function NotesPill({ notes, calendarId, date, onOpenDay }: Props) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const popoverRef = useRef<HTMLDivElement | null>(null);
  const popoverId = useId();

  const close = useCallback(() => setOpen(false), []);

  useEffect(() => {
    if (!open) return;

    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        close();
        triggerRef.current?.focus();
      }
    };

    const handlePointer = (e: PointerEvent) => {
      const t = e.target as Node | null;
      if (!t) return;
      if (popoverRef.current?.contains(t)) return;
      if (triggerRef.current?.contains(t)) return;
      close();
    };

    document.addEventListener("keydown", handleKey);
    document.addEventListener("pointerdown", handlePointer);
    return () => {
      document.removeEventListener("keydown", handleKey);
      document.removeEventListener("pointerdown", handlePointer);
    };
  }, [open, close]);

  if (notes.length === 0) return null;

  function toggle(e: React.MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    setOpen((prev) => {
      const next = !prev;
      if (next && calendarId) {
        analytics.capture("notes_popover_opened", {
          calendar_id: calendarId,
          note_count: notes.length,
          ...(date ? { date } : {}),
        });
      }
      return next;
    });
  }

  function handleRowClick(noteDate: string) {
    if (calendarId) {
      analytics.capture("notes_popover_row_clicked", {
        calendar_id: calendarId,
        ...(noteDate ? { date: noteDate } : {}),
      });
    }
    close();
    onOpenDay?.(noteDate);
  }

  const triggerLabel =
    notes.length === 1
      ? "1 note on this day. Show it."
      : `${notes.length} notes on this day. Show them.`;

  return (
    <span className={styles.wrap}>
      <button
        ref={triggerRef}
        type="button"
        className={styles.pillButton}
        aria-haspopup="true"
        aria-expanded={open}
        aria-controls={open ? popoverId : undefined}
        aria-label={triggerLabel}
        onClick={toggle}
      >
        <span aria-hidden="true" className={styles.glyph}>
          ❝
        </span>
        <span className={styles.label}>
          {notes.length === 1 ? `1 note` : `${notes.length} notes`}
        </span>
      </button>
      {open ? (
        <div
          ref={popoverRef}
          id={popoverId}
          className={styles.popover}
          role="group"
          aria-label="Notes on this day"
        >
          <ul className={styles.list}>
            {notes.map((n) => {
              const rowContent = (
                <>
                  <span
                    aria-hidden="true"
                    className={styles.swatch}
                    style={{ backgroundColor: n.colour }}
                  />
                  <span className={styles.rowBody}>
                    <span className={styles.rowName}>{n.name}</span>
                    <span className={styles.rowText}>{n.text}</span>
                  </span>
                </>
              );

              if (onOpenDay && date) {
                const rowLabel = `Open ${formatDate(date)} to read ${n.name}'s note`;
                return (
                  <li key={n.id}>
                    <button
                      type="button"
                      className={`${styles.row} ${styles.rowButton}`}
                      aria-label={rowLabel}
                      onClick={() => handleRowClick(date)}
                    >
                      {rowContent}
                    </button>
                  </li>
                );
              }

              return (
                <li key={n.id} className={styles.row}>
                  {rowContent}
                </li>
              );
            })}
          </ul>
        </div>
      ) : null}
    </span>
  );
}
