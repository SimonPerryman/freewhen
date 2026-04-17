"use client";

import { useState, useEffect, useRef, useMemo, useCallback, use } from "react";
import { getCalendar, addParticipant, setAvailability, ApiError } from "@/lib/api";
import type { CalendarWithDetails, Participant } from "@/lib/types";
import CalendarGrid from "@/components/CalendarGrid/CalendarGrid";
import JoinForm from "@/components/JoinForm/JoinForm";
import ParticipantList from "@/components/ParticipantList/ParticipantList";
import CopyLinkButton from "@/components/CopyLinkButton/CopyLinkButton";
import SummaryView from "@/components/SummaryView/SummaryView";
import styles from "./page.module.css";

const STORAGE_PREFIX = "freewhen:";

interface StoredParticipant {
  participantId: string;
  name: string;
}

function getToday(): string {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function readStorage(calendarId: string): StoredParticipant | null {
  try {
    const raw = localStorage.getItem(`${STORAGE_PREFIX}${calendarId}`);
    if (!raw) return null;
    return JSON.parse(raw) as StoredParticipant;
  } catch {
    return null;
  }
}

function writeStorage(calendarId: string, data: StoredParticipant): void {
  localStorage.setItem(`${STORAGE_PREFIX}${calendarId}`, JSON.stringify(data));
}

export default function CalendarPage({ params }: { params: Promise<{ id: string }> }) {
  const { id: calendarId } = use(params);

  const [calendar, setCalendar] = useState<CalendarWithDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [currentParticipant, setCurrentParticipant] = useState<Participant | null>(null);
  const [selectedDates, setSelectedDates] = useState<Set<string>>(new Set());
  const [saveError, setSaveError] = useState(false);
  const [joinError, setJoinError] = useState<string | null>(null);
  const [storedName, setStoredName] = useState("");

  // Serialized save refs
  const savingRef = useRef(false);
  const pendingDatesRef = useRef<string[] | null>(null);
  const participantIdRef = useRef<string | null>(null);

  // Keep participantIdRef in sync
  useEffect(() => {
    participantIdRef.current = currentParticipant?.id ?? null;
  }, [currentParticipant]);

  // Fetch calendar data on mount
  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const data = await getCalendar(calendarId);
        if (cancelled) return;
        setCalendar(data);

        // Check localStorage for existing participant
        const stored = readStorage(calendarId);
        if (stored) {
          const existing = data.participants.find((p) => p.id === stored.participantId);
          if (existing) {
            setCurrentParticipant(existing);
            participantIdRef.current = existing.id;

            // Seed selected dates, pruning past dates
            const today = getToday();
            const userDates = data.availability
              .filter((a) => a.participantId === existing.id && a.date >= today)
              .map((a) => a.date);
            setSelectedDates(new Set(userDates));
          } else {
            // Stale participant — pre-fill name
            setStoredName(stored.name);
          }
        }
      } catch (err) {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 404) {
          setError("Calendar not found");
        } else {
          setError("Failed to load calendar");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, [calendarId]);

  // Serialized save logic
  const flushSave = useCallback(async () => {
    savingRef.current = true;
    setSaveError(false);

    while (pendingDatesRef.current !== null) {
      const dates = pendingDatesRef.current;
      pendingDatesRef.current = null;
      const pid = participantIdRef.current;
      if (!pid) break;

      try {
        await setAvailability(calendarId, {
          participantId: pid,
          dates,
        });
      } catch {
        setSaveError(true);
        savingRef.current = false;
        return;
      }
    }

    savingRef.current = false;
  }, [calendarId]);

  const queueSave = useCallback(
    (dates: string[]) => {
      pendingDatesRef.current = dates;
      if (!savingRef.current) flushSave();
    },
    [flushSave],
  );

  const handleToggle = useCallback(
    (date: string) => {
      setSelectedDates((prev) => {
        const next = new Set(prev);
        if (next.has(date)) next.delete(date);
        else next.add(date);
        queueSave(Array.from(next));
        return next;
      });
    },
    [queueSave],
  );

  const handleRetry = useCallback(() => {
    queueSave(Array.from(selectedDates));
  }, [selectedDates, queueSave]);

  const handleJoin = useCallback(
    async (name: string) => {
      setJoinError(null);
      try {
        const participant = await addParticipant(calendarId, { name });
        setCurrentParticipant(participant);
        participantIdRef.current = participant.id;
        writeStorage(calendarId, {
          participantId: participant.id,
          name,
        });
        // Add participant to local calendar state
        setCalendar((prev) =>
          prev
            ? {
                ...prev,
                participants: [...prev.participants, participant],
              }
            : prev,
        );
      } catch (err) {
        if (err instanceof ApiError && err.status === 409) {
          setJoinError("That name is already taken");
        } else {
          setJoinError("Something went wrong");
        }
        throw err; // let JoinForm know submission failed
      }
    },
    [calendarId],
  );

  // Build freeByDate: for each date, the set of participant IDs that are free
  const freeByDate = useMemo(() => {
    const map = new Map<string, Set<string>>();
    if (!calendar) return map;

    // Include current user's optimistic selectedDates
    for (const a of calendar.availability) {
      // Skip current user's backend availability — we use selectedDates instead
      if (currentParticipant && a.participantId === currentParticipant.id) continue;
      const set = map.get(a.date) ?? new Set<string>();
      set.add(a.participantId);
      map.set(a.date, set);
    }

    // Layer in current user's local selection
    if (currentParticipant) {
      for (const date of selectedDates) {
        const set = map.get(date) ?? new Set<string>();
        set.add(currentParticipant.id);
        map.set(date, set);
      }
    }

    return map;
  }, [calendar, currentParticipant, selectedDates]);

  if (loading) {
    return (
      <div className={styles.page}>
        <div className={styles.card}>
          <p className={styles.loading}>Loading…</p>
        </div>
      </div>
    );
  }

  if (error || !calendar) {
    return (
      <div className={styles.page}>
        <div className={styles.card}>
          <p className={styles.error}>{error ?? "Calendar not found"}</p>
        </div>
      </div>
    );
  }

  const calendarName = calendar.calendar.name || "Untitled Calendar";

  return (
    <div className={styles.page}>
      <div className={styles.card}>
        <div className={styles.header}>
          <h1 className={styles.heading}>{calendarName}</h1>
          <CopyLinkButton />
        </div>

        {!currentParticipant ? (
          <JoinForm onJoin={handleJoin} error={joinError} defaultName={storedName} />
        ) : null}

        {saveError ? (
          <div className={styles.banner} role="alert">
            <span>Save failed</span>
            <button type="button" className={styles.retryButton} onClick={handleRetry}>
              Retry
            </button>
          </div>
        ) : null}

        <CalendarGrid
          participants={calendar.participants}
          freeByDate={freeByDate}
          currentParticipantId={currentParticipant?.id ?? null}
          onToggle={handleToggle}
          joined={!!currentParticipant}
        />

        <SummaryView
          participants={calendar.participants}
          freeByDate={freeByDate}
        />

        <ParticipantList
          participants={calendar.participants}
          currentParticipantId={currentParticipant?.id ?? null}
        />
      </div>
    </div>
  );
}
