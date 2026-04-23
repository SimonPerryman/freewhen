"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ackCalendarImportConsent,
  fetchImportSuggestions,
  googleCalendarConnectUrl,
  importAvailabilityDays,
} from "@/lib/api";
import log from "@/lib/log";
import { useToast } from "@/lib/toast";
import type { User } from "@/lib/types";
import styles from "./ImportFromGoogle.module.css";

interface ImportFromGoogleProps {
  calendarId: string;
  user: User | null;
  participantId: string | null;
  onImportComplete?: () => void;
}

interface State {
  free: Set<string>;
  busy: Set<string>;
}

const empty: State = { free: new Set(), busy: new Set() };

export interface ImportSuggestionsView {
  suggestedFree: Set<string>;
  suggestedBusy: Set<string>;
  clearSuggestion: (date: string) => void;
  entry: React.ReactNode;
  chrome: React.ReactNode;
  interstitial: React.ReactNode;
}

// useImportFromGoogle wires together the Google Calendar import flow:
//   • hydrates the single-use suggestion cookie on mount
//   • renders the "Import from Google" entry button (signed-in only)
//   • shows the privacy interstitial before the first OAuth round-trip
//   • shows a progress chip + key strip while suggestions are outstanding
//   • exposes Sets of suggested dates for CalendarGrid to render the dashed /
//     hatched cell states, plus a clearSuggestion helper the tap handler
//     calls after committing a cell through the normal availability endpoint.
export function useImportFromGoogle({
  calendarId,
  user,
  participantId,
  onImportComplete,
}: ImportFromGoogleProps): ImportSuggestionsView {
  const [suggestions, setSuggestions] = useState<State>(empty);
  const [showInterstitial, setShowInterstitial] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const toast = useToast();

  // One-shot hydration: 404 means no pending import, which is the normal
  // path on most page loads.
  useEffect(() => {
    if (!user) return;
    let cancelled = false;
    fetchImportSuggestions(calendarId)
      .then((s) => {
        if (cancelled || !s) return;
        if (s.free.length === 0 && s.busy.length === 0) {
          toast.showToast({ message: "Import expired — try again" });
          return;
        }
        setSuggestions({ free: new Set(s.free), busy: new Set(s.busy) });
      })
      .catch((err: unknown) => {
        log.warn("import suggestions fetch failed", { err });
      });
    return () => {
      cancelled = true;
    };
  }, [calendarId, user, toast]);

  const startOAuth = useCallback(() => {
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
    const returnUrl = `/calendars/${calendarId}`;
    window.location.href = googleCalendarConnectUrl(calendarId, tz, returnUrl);
  }, [calendarId]);

  const handleEntryClick = useCallback(() => {
    if (!user) return;
    if (!user.calendarImportConsentAckAt) {
      setShowInterstitial(true);
      return;
    }
    startOAuth();
  }, [user, startOAuth]);

  const handleInterstitialContinue = useCallback(async () => {
    setSubmitting(true);
    try {
      await ackCalendarImportConsent();
      startOAuth();
    } catch (err) {
      log.error("consent ack failed", { err });
      toast.showToast({ message: "Couldn't record consent — please try again" });
      setSubmitting(false);
    }
  }, [startOAuth, toast]);

  const clearSuggestion = useCallback((date: string) => {
    setSuggestions((prev) => {
      if (!prev.free.has(date) && !prev.busy.has(date)) return prev;
      const free = new Set(prev.free);
      const busy = new Set(prev.busy);
      free.delete(date);
      busy.delete(date);
      return { free, busy };
    });
  }, []);

  const handleAcceptAll = useCallback(async () => {
    if (!participantId || suggestions.free.size === 0) return;
    const days = Array.from(suggestions.free);
    try {
      await importAvailabilityDays(calendarId, participantId, days);
      setSuggestions((prev) => ({ free: new Set(), busy: prev.busy }));
      onImportComplete?.();
      toast.showToast({ message: `Imported ${days.length} free day${days.length === 1 ? "" : "s"}` });
    } catch (err) {
      log.error("accept-all import failed", { err });
      toast.showToast({ message: "Couldn't import — please try again" });
    }
  }, [calendarId, participantId, suggestions.free, onImportComplete, toast]);

  const outstanding = suggestions.free.size + suggestions.busy.size;

  const entry = user ? (
    <button
      type="button"
      className={styles.entryButton}
      onClick={handleEntryClick}
      aria-label="Import free/busy from Google Calendar"
    >
      Import from Google
    </button>
  ) : null;

  const chrome = outstanding > 0 ? (
    <div className={styles.chromeRow}>
      <div className={styles.chip} role="status" aria-live="polite">
        <span className={styles.chipDot} aria-hidden="true" />
        <span className={styles.chipText}>
          Google · {outstanding} of {outstanding} left
        </span>
        {participantId && suggestions.free.size > 0 ? (
          <button type="button" className={styles.acceptAll} onClick={handleAcceptAll}>
            Accept all
          </button>
        ) : null}
      </div>
      <p className={styles.keyStrip}>
        <span>◌ Tap to confirm</span>
        <span>▨ Tap to override</span>
        <span>● Confirmed</span>
      </p>
    </div>
  ) : null;

  const interstitial = showInterstitial ? (
    <div className={styles.interstitialBackdrop} role="dialog" aria-modal="true" aria-labelledby="fw-import-title">
      <div className={styles.interstitial}>
        <h2 id="fw-import-title" className={styles.interstitialTitle}>
          Reading your Google Calendar
        </h2>
        <ul className={styles.interstitialList}>
          <li>We only see whether you&apos;re busy, never event titles or attendees.</li>
          <li>Nothing is stored — the busy times are used once to suggest free days.</li>
          <li>You confirm each day by tapping. Nothing joins the group calendar until you tap.</li>
          <li>Tapping a suggestion makes it part of your availability — revoking Google access later won&apos;t undo it.</li>
        </ul>
        <div className={styles.interstitialActions}>
          <button
            type="button"
            className={styles.interstitialCancel}
            onClick={() => setShowInterstitial(false)}
            disabled={submitting}
          >
            Cancel
          </button>
          <button
            type="button"
            className={styles.interstitialContinue}
            onClick={handleInterstitialContinue}
            disabled={submitting}
          >
            {submitting ? "Continuing…" : "Continue"}
          </button>
        </div>
      </div>
    </div>
  ) : null;

  return {
    suggestedFree: suggestions.free,
    suggestedBusy: suggestions.busy,
    clearSuggestion,
    entry,
    chrome,
    interstitial,
  };
}
