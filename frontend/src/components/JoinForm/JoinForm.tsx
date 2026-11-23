"use client";

import { useState, type FormEvent } from "react";
import { MAX_PARTICIPANT_NAME_LENGTH } from "@/lib/limits";
import CharCounter from "@/components/CharCounter/CharCounter";
import Button from "@/components/Button/Button";
import styles from "./JoinForm.module.css";

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
  const [showEmail, setShowEmail] = useState(false);
  const [nameInvalid, setNameInvalid] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) {
      setNameInvalid(true);
      return;
    }
    const trimmedEmail = email.trim();
    setSubmitting(true);
    try {
      await onJoin(trimmedName, hideEmail || !showEmail ? null : trimmedEmail || null);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className={styles.form} onSubmit={handleSubmit} noValidate>
      <label className={styles.label} htmlFor="participant-name">
        Your name
      </label>
      <input
        id="participant-name"
        className={styles.input}
        type="text"
        value={name}
        onChange={(e) => {
          setName(e.target.value);
          setNameInvalid(false);
        }}
        placeholder="Enter your name"
        autoComplete="off"
        disabled={submitting}
        maxLength={MAX_PARTICIPANT_NAME_LENGTH}
        aria-invalid={!!error || nameInvalid}
        aria-describedby={error || nameInvalid ? "participant-name-error" : undefined}
      />
      <CharCounter count={name.length} max={MAX_PARTICIPANT_NAME_LENGTH} />
      {error || nameInvalid ? (
        <p id="participant-name-error" role="alert" className={styles.error}>
          {error ?? "Enter your name to join."}
        </p>
      ) : null}
      {!hideEmail && !showEmail ? (
        <button type="button" className={styles.emailLink} onClick={() => setShowEmail(true)}>
          + Get notified when a date&apos;s picked
        </button>
      ) : null}
      {!hideEmail && showEmail ? (
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
            autoFocus
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
        disabled={submitting}
      >
        {submitting ? "Joining…" : "Join"}
      </Button>
    </form>
  );
}
