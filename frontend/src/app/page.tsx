"use client";

import { useState, useEffect, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { ApiError, createCalendar } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import styles from "./page.module.css";

export default function Home() {
  const router = useRouter();
  const { user, isLoading, checkAuth } = useAuth();
  const [name, setName] = useState("");
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
      const calendar = await createCalendar({ name: name.trim() });
      router.push(`/calendars/${calendar.id}`);
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
