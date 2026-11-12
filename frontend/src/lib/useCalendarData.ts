"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
import { useRouter } from "next/navigation";
import { getCalendar, unlockCalendar, ApiError, CalendarUnavailableError } from "./api";
import { clearCalendarAccessToken } from "./calendarAccess";
import analytics from "./analytics";
import { useToast } from "./toast";
import type { CalendarWithDetails, User } from "./types";

// How long after one of our own writes to ignore live-update refreshes, so
// our own SSE echo doesn't clobber optimistic state.
export const LOCAL_MUTATION_SUPPRESS_MS = 500;

export type LoadError = "not-found" | "load-failed";

export interface UseCalendarDataOptions {
  // Signed-in user (or null). Captured when a load starts, like the deps of
  // the original load effect — a user who resolves mid-load doesn't re-run it.
  user: User | null;
  // Runs after every full load, once the calendar is in state and before
  // `loading` flips false. Awaited, so participant resolution (including the
  // silent claim) finishes behind the loading screen.
  onLoaded?: (data: CalendarWithDetails) => void | Promise<void>;
  // Runs after every successful live-update refresh.
  onRefreshed?: (data: CalendarWithDetails) => void;
}

export interface UseCalendarDataResult {
  calendar: CalendarWithDetails | null;
  setCalendar: Dispatch<SetStateAction<CalendarWithDetails | null>>;
  loading: boolean;
  error: LoadError | null;
  requiresPassphrase: boolean;
  magicLinkError: string | null;
  // Re-run the full load (error-panel "Try again").
  retryLoad: () => void;
  // Called by the passphrase gate once the calendar is unlocked.
  handleUnlocked: () => void;
  // Lightweight refetch for live updates; skipped while a local write is
  // settling (see markLocalMutation).
  refresh: () => Promise<void>;
  // Call before/after a local write to suppress our own SSE echo.
  markLocalMutation: () => void;
}

/**
 * Owns the calendar fetched for `/calendars/[id]`: the full load (including
 * the `?unlock=` magic-link handshake), loading / error / passphrase-gate
 * state, and the lightweight refresh used by the live-updates stream.
 */
export function useCalendarData(
  calendarId: string,
  options: UseCalendarDataOptions,
): UseCalendarDataResult {
  const { showToast } = useToast();
  const router = useRouter();
  const [calendar, setCalendar] = useState<CalendarWithDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<LoadError | null>(null);
  const [requiresPassphrase, setRequiresPassphrase] = useState(false);
  const [magicLinkError, setMagicLinkError] = useState<string | null>(null);
  const suppressRefreshUntilRef = useRef(0);

  // Latest options, read at the *start* of a load / refresh. Layout effects
  // commit before passive effects, so the load effect below sees the options
  // from the same render — matching a plain closure capture.
  const optionsRef = useRef(options);
  useLayoutEffect(() => {
    optionsRef.current = options;
  });

  const markLocalMutation = useCallback(() => {
    suppressRefreshUntilRef.current = Date.now() + LOCAL_MUTATION_SUPPRESS_MS;
  }, []);

  const [loadAttempt, setLoadAttempt] = useState(0);
  const retryLoad = useCallback(() => {
    setLoadAttempt((n) => n + 1);
  }, []);

  useEffect(() => {
    let cancelled = false;
    const { user, onLoaded } = optionsRef.current;
    setLoading(true);
    setError(null);

    async function load() {
      try {
        // Handle `?unlock=<passphrase>` magic-link before the first API call
        // so subsequent requests carry the access token.
        if (typeof window !== "undefined") {
          const url = new URL(window.location.href);
          const unlock = url.searchParams.get("unlock");
          if (unlock) {
            // Strip the param immediately so it doesn't persist in history.
            url.searchParams.delete("unlock");
            window.history.replaceState({}, "", url.toString());
            try {
              await unlockCalendar(calendarId, unlock);
              analytics.capture("calendar_unlock_attempted", {
                calendar_id: calendarId,
                via: "magic_link",
                success: true,
              });
            } catch {
              analytics.capture("calendar_unlock_attempted", {
                calendar_id: calendarId,
                via: "magic_link",
                success: false,
              });
              if (!cancelled) setMagicLinkError("That link is no longer valid.");
            }
          }
        }

        const data = await getCalendar(calendarId);
        if (cancelled) return;

        if (data.requiresPassphrase) {
          setRequiresPassphrase(true);
          return;
        }
        setRequiresPassphrase(false);
        setCalendar(data);
        analytics.capture("calendar_opened", {
          calendar_id: calendarId,
          is_owner: !!user && data.calendar.ownerId === user.id,
        });

        await onLoaded?.(data);
      } catch (err) {
        if (cancelled) return;
        if (err instanceof CalendarUnavailableError) {
          showToast({ message: "This calendar is no longer available." });
          router.replace("/calendars");
          return;
        }
        if (err instanceof ApiError && err.status === 404) {
          setError("not-found");
        } else {
          setError("load-failed");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
    // Deliberately keyed on the calendar and explicit retries only — auth
    // resolving later does not re-run the load.
  }, [calendarId, loadAttempt]);

  // Lightweight refresh used by the live-updates stream. Unlike the full
  // load it does not re-run the claim flow or re-seed selected dates — those
  // belong to initial mount. Local-mutation suppression prevents our own
  // writes from flickering on echo.
  const refresh = useCallback(async () => {
    if (Date.now() < suppressRefreshUntilRef.current) return;
    try {
      const data = await getCalendar(calendarId);
      if (data.requiresPassphrase) {
        clearCalendarAccessToken(calendarId);
        setCalendar(null);
        setRequiresPassphrase(true);
        return;
      }
      setCalendar(data);
      optionsRef.current.onRefreshed?.(data);
    } catch (err) {
      if (err instanceof CalendarUnavailableError) {
        showToast({ message: "This calendar is no longer available." });
        router.replace("/calendars");
        return;
      }
      // Transient refresh failure; next event or the polling fallback retries.
    }
  }, [calendarId, showToast, router]);

  const handleUnlocked = useCallback(() => {
    setMagicLinkError(null);
    setRequiresPassphrase(false);
    retryLoad();
  }, [retryLoad]);

  return {
    calendar,
    setCalendar,
    loading,
    error,
    requiresPassphrase,
    magicLinkError,
    retryLoad,
    handleUnlocked,
    refresh,
    markLocalMutation,
  };
}
