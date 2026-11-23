"use client";

import { useState, type FormEvent } from "react";
import { ApiError, unlockCalendar } from "@/lib/api";
import Button from "@/components/Button/Button";
import TextField from "@/components/TextField/TextField";
import styles from "./PassphraseGate.module.css";

interface PassphraseGateProps {
  calendarId: string;
  onUnlocked: () => void;
  // When the caller already tried a passphrase (e.g. via `?unlock=`) and it
  // was rejected, surface that as the initial error so the user isn't left
  // wondering why they're seeing the gate.
  initialError?: string | null;
}

export default function PassphraseGate({
  calendarId,
  onUnlocked,
  initialError = null,
}: PassphraseGateProps) {
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(initialError);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = value.trim();
    if (!trimmed) return;
    setSubmitting(true);
    setError(null);
    try {
      await unlockCalendar(calendarId, trimmed);
      onUnlocked();
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 429) {
          setError("Too many attempts — try again in a minute.");
        } else if (err.status === 401) {
          setError("That passphrase doesn't match. Double-check and try again.");
        } else {
          setError(err.message || "Couldn't unlock — try again.");
        }
      } else {
        setError("Couldn't unlock — try again.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className={styles.panel}>
      <h1 className={styles.heading}>This calendar is private</h1>
      <p className={styles.sub}>Enter the passphrase you were given to continue.</p>
      <form className={styles.form} onSubmit={handleSubmit}>
        <TextField
          label="Passphrase"
          hideLabel
          placeholder="correct-horse-battery-staple"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          autoComplete="off"
          autoFocus
          disabled={submitting}
          required
          aria-required="true"
          error={error}
        />
        <Button type="submit" variant="primary" size="lg" disabled={submitting || !value.trim()}>
          {submitting ? "Unlocking…" : "Unlock"}
        </Button>
      </form>
    </div>
  );
}
