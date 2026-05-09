"use client";

import { useState, type FormEvent } from "react";
import { ApiError, unlockCalendar } from "@/lib/api";
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
        <input
          type="text"
          className={styles.input}
          placeholder="correct-horse-battery-staple"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          autoComplete="off"
          autoFocus
          disabled={submitting}
          required
          aria-required="true"
          aria-label="Passphrase"
          aria-invalid={!!error}
          aria-describedby={error ? "passphrase-gate-error" : undefined}
        />
        <button type="submit" className={styles.button} disabled={submitting || !value.trim()}>
          {submitting ? "Unlocking…" : "Unlock →"}
        </button>
        {error ? (
          <p id="passphrase-gate-error" className={styles.error} role="alert">
            {error}
          </p>
        ) : null}
      </form>
    </div>
  );
}
