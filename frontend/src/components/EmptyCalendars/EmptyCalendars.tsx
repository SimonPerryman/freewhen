"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { ApiError, createCalendar } from "@/lib/api";
import { MAX_CALENDAR_NAME_LENGTH } from "@/lib/limits";
import analytics from "@/lib/analytics";
import Button from "@/components/Button/Button";
import TextField from "@/components/TextField/TextField";
import styles from "./EmptyCalendars.module.css";

// Shown on My calendars when there is nothing to list: a pitch plus an inline
// create form, so the first calendar is one step away.
export default function EmptyCalendars() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) {
      setError("Give your calendar a name.");
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      const result = await createCalendar({ name: trimmed });
      analytics.capture("calendar_created", { calendar_id: result.id });
      if (result.participant) {
        localStorage.setItem(
          `freewhen:${result.id}`,
          JSON.stringify({
            participantId: result.participant.id,
            name: result.participant.name,
            colour: result.participant.colour,
          }),
        );
      }
      router.push(`/calendars/${result.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Something went wrong");
      setSubmitting(false);
    }
  }

  return (
    <section className={styles.empty} aria-labelledby="empty-calendars-title">
      <div className={styles.grid} aria-hidden="true">
        {Array.from({ length: 21 }, (_, i) => (
          <span key={i} className={[styles.cell, FREE_CELLS.has(i) ? styles.free : ""].join(" ")} />
        ))}
      </div>
      <h2 id="empty-calendars-title" className={styles.title}>
        No calendars yet
      </h2>
      <p className={styles.pitch}>
        Name an event, share the link, and see the dates everyone is free.
      </p>
      <form className={styles.form} onSubmit={handleSubmit} noValidate>
        <TextField
          className={styles.field}
          label="Event name"
          hideLabel
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Weekend trip"
          autoComplete="off"
          maxLength={MAX_CALENDAR_NAME_LENGTH}
          disabled={submitting}
          error={error ?? undefined}
        />
        <Button type="submit" variant="primary" size="lg" loading={submitting}>
          {submitting ? "Creating…" : "Create"}
        </Button>
      </form>
    </section>
  );
}

const FREE_CELLS = new Set([2, 3, 8, 9, 10, 15, 16]);
