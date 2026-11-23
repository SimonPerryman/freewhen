"use client";

import { useState, type FormEvent } from "react";
import { MAX_PARTICIPANT_NAME_LENGTH } from "@/lib/limits";
import Button from "@/components/Button/Button";
import styles from "./JoinForm.module.css";

// Show a live character count once the user is within this many characters of the cap.
const COUNTER_THRESHOLD = 10;

interface JoinFormProps {
  onJoin: (name: string, email?: string | null) => Promise<void>;
  error: string | null;
  defaultName?: string;
  // hideEmail skips the optional email field entirely. Set for signed-in
  // joiners (we already have their account email — no need to re-ask).
  hideEmail?: boolean;
}

export default function JoinForm({
  onJoin,
  error,
  defaultName = "",
  hideEmail = false,
}: JoinFormProps) {
  const [name, setName] = useState(defaultName);
  const [email, setEmail] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) return;
    const trimmedEmail = email.trim();
    setSubmitting(true);
    try {
      await onJoin(trimmedName, hideEmail ? null : trimmedEmail || null);
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
        required
        aria-required="true"
        aria-invalid={!!error}
        aria-describedby={error ? "participant-name-error" : undefined}
      />
      {showCounter ? (
        <p className={styles.counter} aria-live="polite">
          {name.length} / {MAX_PARTICIPANT_NAME_LENGTH}
        </p>
      ) : null}
      {error ? (
        <p id="participant-name-error" role="alert" className={styles.error}>
          {error}
        </p>
      ) : null}
      {!hideEmail ? (
        <div className={styles.emailField}>
          <label className={styles.label} htmlFor="participant-email">
            Email (optional)
          </label>
          <input
            id="participant-email"
            className={styles.input}
            type="email"
            inputMode="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
            disabled={submitting}
            aria-describedby="participant-email-helper"
          />
          <p id="participant-email-helper" className={styles.helper}>
            Only used to tell you when a date&apos;s picked.
          </p>
        </div>
      ) : null}
      {/* Submit comes after every field so tab order and reading order match. */}
      <Button
        type="submit"
        variant="primary"
        size="lg"
        className={styles.submit}
        disabled={submitting || !name.trim()}
      >
        {submitting ? "Joining…" : "Join"}
      </Button>
    </form>
  );
}
