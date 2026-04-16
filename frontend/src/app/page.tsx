"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { ApiError, createCalendar } from "@/lib/api";
import styles from "./page.module.css";

export default function Home() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const calendar = await createCalendar({ name: name.trim() });
      router.push(`/calendars/${calendar.id}`);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Something went wrong";
      setError(message);
      setSubmitting(false);
    }
  }

  return (
    <div className={styles.page}>
      <main className={styles.card}>
        <h1 className={styles.heading}>Create a calendar</h1>
        <p className={styles.subheading}>Share a link and collect availability from your group.</p>
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
          <button type="submit" className={styles.button} disabled={submitting}>
            {submitting ? "Creating…" : "Create calendar"}
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
