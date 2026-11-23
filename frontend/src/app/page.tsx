"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { ApiError, createCalendar } from "@/lib/api";
import { MAX_CALENDAR_NAME_LENGTH } from "@/lib/limits";
import analytics from "@/lib/analytics";
import { savePendingCalendarName, takePendingCalendarName } from "@/lib/pendingCalendarName";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import HeroLiveGrid from "@/components/HeroLiveGrid/HeroLiveGrid";
import { useTypewriterPlaceholder } from "@/lib/useTypewriterPlaceholder";
import styles from "./page.module.css";

const PLACEHOLDER_PHRASES = [
  "Weekend trip",
  "Sarah's birthday",
  "Team offsite",
  "Ski week",
  "Climbing weekend",
] as const;

export default function Home() {
  const router = useRouter();
  const { user, isLoading } = useAuth();
  const [name, setName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isFocused, setIsFocused] = useState(false);
  const placeholder = useTypewriterPlaceholder(PLACEHOLDER_PHRASES, {
    paused: name.length > 0 || isFocused,
  });

  async function create(calendarName: string) {
    setError(null);
    setSubmitting(true);
    try {
      const result = await createCalendar({ name: calendarName });
      analytics.capture("calendar_created", { calendar_id: result.id });

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
        savePendingCalendarName(calendarName);
        router.push("/login");
        return;
      }
      let message = "Something went wrong";
      if (err instanceof ApiError) {
        if (err.code === "calendar_limit_reached") {
          message =
            "You've reached the 200-calendar limit. Delete an old calendar to create a new one.";
        } else {
          message = err.message;
        }
      }
      setError(message);
      setSubmitting(false);
    }
  }

  // Back from the magic-link sign-in with a name saved on the way out: create
  // the calendar now. The name is removed from storage before the request, so
  // a failure leaves it in the field to retry and a reload creates nothing.
  const resumed = useRef(false);
  useEffect(() => {
    if (isLoading || !user || resumed.current) return;
    resumed.current = true;
    const pending = takePendingCalendarName();
    if (pending === null) return;
    setName(pending);
    void create(pending);
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- runs once per sign-in
  }, [isLoading, user]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = name.trim();

    if (!user) {
      if (trimmed !== "") savePendingCalendarName(trimmed);
      router.push("/login");
      return;
    }

    void create(trimmed);
  }

  const buttonLabel = submitting ? "Creating…" : "Create";

  return (
    <div className={styles.page}>
      <main id="main" className={styles.container}>
        <h1 className={styles.headline}>
          Find when
          <br />
          everyone&rsquo;s <em className="serif-accent">free.</em>
        </h1>

        <p className={styles.subheading}>Drop a name, share a link, watch availability fill in.</p>

        <HeroLiveGrid />

        <form className={styles.form} onSubmit={handleSubmit}>
          <label className={styles.label} htmlFor="calendar-name">
            Event name
          </label>
          <div className={styles.cta}>
            <input
              id="calendar-name"
              className={styles.input}
              type="text"
              value={name}
              onChange={(event) => setName(event.target.value)}
              onFocus={() => setIsFocused(true)}
              onBlur={() => setIsFocused(false)}
              placeholder={placeholder}
              autoComplete="off"
              disabled={submitting}
              maxLength={MAX_CALENDAR_NAME_LENGTH}
            />
            <button type="submit" className={styles.button} disabled={submitting || isLoading}>
              {buttonLabel}
            </button>
          </div>
          {!user && !isLoading ? (
            <p className={styles.hint}>You&rsquo;ll sign in with your email to save it.</p>
          ) : null}
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
