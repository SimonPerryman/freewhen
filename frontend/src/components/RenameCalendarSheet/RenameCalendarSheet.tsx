"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, renameCalendar } from "@/lib/api";
import analytics from "@/lib/analytics";
import { MAX_CALENDAR_NAME_LENGTH } from "@/lib/limits";
import Button from "@/components/Button/Button";
import Sheet from "@/components/Sheet/Sheet";
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

  useEffect(() => {
    inputRef.current?.select();
  }, []);

  // The submit button is disabled mid-request, which drops focus; put it back
  // on the field once an error re-enables the form.
  useEffect(() => {
    if (error) inputRef.current?.focus();
  }, [error]);

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
    <Sheet
      title="Rename calendar"
      onClose={onClose}
      dismissible={!submitting}
      initialFocus={inputRef}
    >
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
    </Sheet>
  );
}
