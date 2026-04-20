"use client";

import { useState, type FormEvent } from "react";
import { MAX_PARTICIPANT_NAME_LENGTH } from "@/lib/limits";
import styles from "./JoinForm.module.css";

// Show a live character count once the user is within this many characters of the cap.
const COUNTER_THRESHOLD = 10;

interface JoinFormProps {
  onJoin: (name: string) => Promise<void>;
  error: string | null;
  defaultName?: string;
}

export default function JoinForm({ onJoin, error, defaultName = "" }: JoinFormProps) {
  const [name, setName] = useState(defaultName);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) return;
    setSubmitting(true);
    try {
      await onJoin(trimmed);
    } finally {
      setSubmitting(false);
    }
  }

  const showCounter = name.length >= MAX_PARTICIPANT_NAME_LENGTH - COUNTER_THRESHOLD;

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <label className={styles.label} htmlFor="participant-name">
        Your name
      </label>
      <div className={styles.row}>
        <input
          id="participant-name"
          className={styles.input}
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Enter your name"
          autoComplete="off"
          disabled={submitting}
          maxLength={MAX_PARTICIPANT_NAME_LENGTH}
        />
        <button type="submit" className={styles.button} disabled={submitting || !name.trim()}>
          {submitting ? "Joining…" : "Join"}
        </button>
      </div>
      {showCounter ? (
        <p className={styles.counter} aria-live="polite">
          {name.length} / {MAX_PARTICIPANT_NAME_LENGTH}
        </p>
      ) : null}
      {error ? (
        <p role="alert" className={styles.error}>
          {error}
        </p>
      ) : null}
    </form>
  );
}
