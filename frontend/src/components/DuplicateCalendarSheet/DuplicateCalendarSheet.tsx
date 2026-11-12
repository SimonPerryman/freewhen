"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, duplicateCalendar } from "@/lib/api";
import analytics from "@/lib/analytics";
import { MAX_CALENDAR_NAME_LENGTH } from "@/lib/limits";
import Button from "@/components/Button/Button";
import Sheet from "@/components/Sheet/Sheet";
import TextField from "@/components/TextField/TextField";
import styles from "./DuplicateCalendarSheet.module.css";

interface Props {
  calendarId: string;
  sourceName: string;
  onClose: () => void;
  onDuplicated: (newCalendarId: string) => void;
}

export default function DuplicateCalendarSheet({
  calendarId,
  sourceName,
  onClose,
  onDuplicated,
}: Props) {
  const defaultName = `${sourceName} (copy)`.slice(0, MAX_CALENDAR_NAME_LENGTH);
  const [name, setName] = useState(defaultName);
  const [includeParticipants, setIncludeParticipants] = useState(true);
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
      setError(null);
      setSubmitting(true);
      const trimmed = name.trim();
      try {
        const resp = await duplicateCalendar(calendarId, {
          name: trimmed === "" ? undefined : trimmed,
          includeParticipants,
        });
        analytics.capture("calendar_duplicated", {
          source_id: calendarId,
          include_participants: includeParticipants,
        });
        onDuplicated(resp.id);
      } catch (err) {
        if (err instanceof ApiError) {
          if (err.code === "calendar_limit_reached") {
            setError("You've reached the calendar limit.");
          } else if (err.code === "name_too_long") {
            setError("That name is too long.");
          } else {
            setError("Something went wrong. Try again.");
          }
          analytics.capture("calendar_duplicate_failed", {
            source_id: calendarId,
            reason: err.code ?? "unknown",
          });
        } else {
          setError("Something went wrong. Try again.");
          analytics.capture("calendar_duplicate_failed", {
            source_id: calendarId,
            reason: "unknown",
          });
        }
        setSubmitting(false);
      }
    },
    [calendarId, name, includeParticipants, onDuplicated],
  );

  return (
    <Sheet
      title="Duplicate this calendar"
      onClose={onClose}
      closeLabel="Close duplicate calendar"
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
          aria-invalid={!!error}
          aria-describedby={error ? "duplicate-sheet-error" : undefined}
        />
        <label className={styles.checkboxRow}>
          <input
            type="checkbox"
            checked={includeParticipants}
            onChange={(e) => setIncludeParticipants(e.target.checked)}
            disabled={submitting}
          />
          <span>
            <strong>Copy participants</strong>
            <span className={styles.hint}>
              Signed-in participants will be added to the new one. Others can rejoin via the link.
              Everyone will need to re-mark availability.
            </span>
          </span>
        </label>
        {error ? (
          <p id="duplicate-sheet-error" className={styles.error} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.actions}>
          <Button variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={submitting}>
            {submitting ? "Duplicating…" : "Duplicate →"}
          </Button>
        </div>
      </form>
    </Sheet>
  );
}
