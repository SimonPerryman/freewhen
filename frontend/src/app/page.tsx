"use client";

import { useState, useEffect, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { ApiError, createCalendar } from "@/lib/api";
import analytics from "@/lib/analytics";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import styles from "./page.module.css";

export default function Home() {
  const router = useRouter();
  const { user, isLoading, checkAuth } = useAuth();
  const [name, setName] = useState("");
  const [hideUntilRespond, setHideUntilRespond] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    if (!user) {
      router.push(`/login?return_url=${encodeURIComponent("/")}`);
      return;
    }

    setSubmitting(true);
    try {
      const visibility = hideUntilRespond ? "submit_to_view" : "open";
      const result = await createCalendar({ name: name.trim(), visibility });
      analytics.capture("calendar_created", { calendar_id: result.id });
      analytics.capture("calendar_visibility_set", {
        calendar_id: result.id,
        visibility,
      });

      // Store owner's auto-joined participant in localStorage
      if (result.participant) {
        const storageKey = `freewhen:${result.id}`;
        localStorage.setItem(
          storageKey,
          JSON.stringify({
            participantId: result.participant.id,
            name: result.participant.name,
            colour: result.participant.colour,
          }),
        );
      }

      router.push(`/calendars/${result.id}`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        router.push(`/login?return_url=${encodeURIComponent("/")}`);
        return;
      }
      const message = err instanceof ApiError ? err.message : "Something went wrong";
      setError(message);
      setSubmitting(false);
    }
  }

  return (
    <div className={styles.page}>
      <main className={styles.container}>
        <h1 className={styles.heading}>FreeWhen</h1>
        <p className={styles.subheading}>Find when everyone&apos;s free.</p>

        <div className={styles.preview} aria-hidden="true">
          <div className={styles.previewGrid}>
            {Array.from({ length: 7 }, (_, i) => (
              <div
                key={i}
                className={`${styles.previewCell} ${i === 2 || i === 5 ? styles.previewCellHighlight : ""}`}
              />
            ))}
          </div>
        </div>

        <form className={styles.form} onSubmit={handleSubmit}>
          <label className={styles.label} htmlFor="calendar-name">
            Event name
          </label>
          <input
            id="calendar-name"
            className={styles.input}
            type="text"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="Weekend trip"
            autoComplete="off"
            disabled={submitting}
          />
          <label className={styles.toggleRow}>
            <input
              type="checkbox"
              className={styles.checkbox}
              checked={hideUntilRespond}
              onChange={(event) => setHideUntilRespond(event.target.checked)}
              disabled={submitting}
            />
            <span>
              <span className={styles.toggleLabel}>Hide others until you respond</span>
              <span className={styles.toggleHelp}>
                Stops people matching the group&apos;s dates instead of picking their own.
              </span>
            </span>
          </label>
          <button type="submit" className={styles.button} disabled={submitting || isLoading}>
            {submitting ? "Creating…" : user ? "Create calendar" : "Sign in to create"}
          </button>
          {error ? (
            <p role="alert" className={styles.error}>
              {error}
            </p>
          ) : null}
        </form>
      </main>
    </div>
  );
}
