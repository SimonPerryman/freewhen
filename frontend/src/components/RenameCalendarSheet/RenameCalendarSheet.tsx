"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, renameCalendar } from "@/lib/api";
import analytics from "@/lib/analytics";
import { MAX_CALENDAR_NAME_LENGTH } from "@/lib/limits";
import { useFocusTrap } from "@/lib/useFocusTrap";
import Button from "@/components/Button/Button";
import TextField from "@/components/TextField/TextField";
import styles from "./RenameCalendarSheet.module.css";

interface Props {
  calendarId: string;
  currentName: string;
  onClose: () => void;
  onRenamed: (newName: string) => void;
}

export default function RenameCalendarSheet({
  calendarId,
  currentName,
  onClose,
  onRenamed,
}: Props) {
  const [name, setName] = useState(currentName);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const sheetRef = useFocusTrap<HTMLDivElement>({
    enabled: !submitting,
    onEscape: () => {
      if (!submitting) onClose();
    },
    initialFocus: inputRef,
  });

  useEffect(() => {
    inputRef.current?.select();
  }, []);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const trimmed = name.trim();
      if (trimmed === "" || trimmed === currentName) {
        onClose();
        return;
      }
      setError(null);
      setSubmitting(true);
      try {
        const updated = await renameCalendar(calendarId, trimmed);
        analytics.capture("calendar_renamed", { calendar_id: calendarId });
        onRenamed(updated.name);
      } catch (err) {
        if (err instanceof ApiError && err.code === "name_too_long") {
          setError("That name is too long.");
        } else {
          setError("Something went wrong. Try again.");
        }
        setSubmitting(false);
      }
    },
    [calendarId, currentName, name, onClose, onRenamed],
  );

  return (
    <div
      className={styles.backdrop}
      onClick={(e) => {
        if (e.target === e.currentTarget && !submitting) onClose();
      }}
    >
      <div
        ref={sheetRef}
        className={styles.sheet}
        role="dialog"
        aria-modal="true"
        aria-labelledby="rename-sheet-title"
      >
        <h2 id="rename-sheet-title" className={styles.heading}>
          Rename calendar
        </h2>
        <form onSubmit={handleSubmit} className={styles.form}>
          <TextField
            ref={inputRef}
            label="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={MAX_CALENDAR_NAME_LENGTH}
            disabled={submitting}
            error={error}
          />
          <div className={styles.actions}>
            <Button variant="secondary" onClick={onClose} disabled={submitting}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" disabled={submitting}>
              {submitting ? "Saving…" : "Save →"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
