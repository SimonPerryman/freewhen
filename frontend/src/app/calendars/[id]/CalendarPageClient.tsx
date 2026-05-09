"use client";

import { useState, useEffect, useRef, useMemo, useCallback } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  getCalendar,
  addParticipant,
  setAvailability,
  finaliseCalendar,
  unfinaliseCalendar,
  claimParticipant,
  removeParticipant,
  renameParticipant,
  renameCalendar,
  unlockCalendar,
  rotateCalendarPassphrase,
  clearCalendarPassphrase,
  ApiError,
} from "@/lib/api";
import { clearCalendarAccessToken } from "@/lib/calendarAccess";
import PassphraseGate from "@/components/PassphraseGate/PassphraseGate";
import analytics from "@/lib/analytics";
import { useCalendarEvents } from "@/lib/useCalendarEvents";
import {
  MAX_CALENDAR_NAME_LENGTH,
  MAX_PARTICIPANT_NAME_LENGTH,
  MAX_PARTICIPANTS_PER_CALENDAR,
} from "@/lib/limits";
import type { CalendarWithDetails, Participant } from "@/lib/types";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import { useToast } from "@/lib/toast";
import CalendarGrid from "@/components/CalendarGrid/CalendarGrid";
import DayDetailSheet from "@/components/DayDetailSheet/DayDetailSheet";
import DuplicateCalendarSheet from "@/components/DuplicateCalendarSheet/DuplicateCalendarSheet";
import FinalisedBanner from "@/components/FinalisedBanner/FinalisedBanner";
import { useImportFromGoogle } from "@/components/ImportFromGoogle/ImportFromGoogle";
import JoinForm from "@/components/JoinForm/JoinForm";
import InvitePrompt from "@/components/InvitePrompt/InvitePrompt";
import ParticipantList from "@/components/ParticipantList/ParticipantList";
import ShareButton from "@/components/ShareButton/ShareButton";
import SummaryView from "@/components/SummaryView/SummaryView";
import BestDay from "@/components/BestDay/BestDay";
import { renderTitle, titlePlain } from "@/lib/title";
import LiveStatusDot from "@/components/LiveStatusDot/LiveStatusDot";
import CalendarSettingsSheet from "@/components/CalendarSettingsSheet/CalendarSettingsSheet";
import ConfirmDialog from "@/components/ConfirmDialog/ConfirmDialog";
import WhosMissingPanel from "@/components/WhosMissingPanel/WhosMissingPanel";
import styles from "./page.module.css";

const LOCAL_MUTATION_SUPPRESS_MS = 500;

type LoadError = "not-found" | "load-failed";

const STORAGE_PREFIX = "freewhen:";
const LAST_NAME_KEY = "freewhen:lastName";

interface StoredParticipant {
  participantId: string;
  name: string;
  colour?: string;
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

function readLastName(): string {
  try {
    const raw = localStorage.getItem(LAST_NAME_KEY);
    if (!raw) return "";
    return raw.slice(0, MAX_PARTICIPANT_NAME_LENGTH);
  } catch {
    return "";
  }
}

function writeLastName(name: string): void {
  try {
    localStorage.setItem(LAST_NAME_KEY, name);
  } catch {
    // ignore quota / disabled storage
  }
}

export default function CalendarPageClient({ calendarId }: { calendarId: string }) {
  const { user } = useAuth();
  const { showToast } = useToast();
  const router = useRouter();
  const [duplicateOpen, setDuplicateOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [removePassphraseOpen, setRemovePassphraseOpen] = useState(false);
  const [regeneratePassphraseOpen, setRegeneratePassphraseOpen] = useState(false);
  const [participantToRemove, setParticipantToRemove] = useState<Participant | null>(null);
  const [calendar, setCalendar] = useState<CalendarWithDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<LoadError | null>(null);
  const [requiresPassphrase, setRequiresPassphrase] = useState(false);
  const [magicLinkError, setMagicLinkError] = useState<string | null>(null);
  const [currentParticipant, setCurrentParticipant] = useState<Participant | null>(null);
  const [selectedDates, setSelectedDates] = useState<Set<string>>(new Set());
  // notesByDate holds the current participant's own notes keyed by ISO date.
  // Hydrated from the backend on load; updated optimistically by the day sheet.
  const [notesByDate, setNotesByDate] = useState<Map<string, string>>(new Map());
  // votesByDate holds the current participant's own preferred-date set. Hydrated
  // from the backend on load; mutated optimistically by the day sheet.
  const [votesByDate, setVotesByDate] = useState<Set<string>>(new Set());
  const [openDayDate, setOpenDayDate] = useState<string | null>(null);
  const [saveError, setSaveError] = useState(false);
  const [joinError, setJoinError] = useState<string | null>(null);
  const [storedName, setStoredName] = useState("");

  // Serialized save refs
  const savingRef = useRef(false);
  const pendingDatesRef = useRef<string[] | null>(null);
  // Coalesces note edits across rapid saves. When null the request omits
  // `notes` entirely (backend's "don't touch" path). Keys here are applied
  // atomically with the next save then cleared.
  const pendingNotesRef = useRef<Record<string, string> | null>(null);
  // Signals that a vote was edited since the last save. Flipped back to false
  // once the write goes out. When false, the save omits `preferred` entirely so
  // the server preserves existing votes.
  const pendingPreferredRef = useRef(false);
  const votesByDateRef = useRef<Set<string>>(votesByDate);
  useEffect(() => {
    votesByDateRef.current = votesByDate;
  }, [votesByDate]);
  const participantIdRef = useRef<string | null>(null);
  const currentParticipantRef = useRef<Participant | null>(null);
  const suppressRefreshUntilRef = useRef(0);
  // Mirrors selectedDates so handlers can compute the next set outside of a
  // setState updater — the updater must stay pure (StrictMode invokes it twice).
  const selectedDatesRef = useRef<Set<string>>(selectedDates);
  useEffect(() => {
    selectedDatesRef.current = selectedDates;
  }, [selectedDates]);

  // Keep participantIdRef in sync
  useEffect(() => {
    participantIdRef.current = currentParticipant?.id ?? null;
    currentParticipantRef.current = currentParticipant;
  }, [currentParticipant]);

  const markLocalMutation = useCallback(() => {
    suppressRefreshUntilRef.current = Date.now() + LOCAL_MUTATION_SUPPRESS_MS;
  }, []);

  // Fetch calendar data
  const [loadAttempt, setLoadAttempt] = useState(0);
  const retryLoad = useCallback(() => {
    setLoadAttempt((n) => n + 1);
  }, []);

  useEffect(() => {
    let cancelled = false;
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

        const stored = readStorage(calendarId);
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

        // Resolve participant: user_id match first, then localStorage
        let resolved: (typeof data.participants)[0] | undefined;
        let prefillName: string | null = null;

        // 1. Auto-reconnect: if signed in, find participant linked to this user
        if (user) {
          resolved = data.participants.find((p) => p.userId === user.id);
        }

        // 2. Fall back to localStorage
        if (!resolved && stored) {
          const existing = data.participants.find((p) => p.id === stored.participantId);
          if (existing) {
            // 3. Claim flow: localStorage participant exists, user is signed in,
            //    but participant isn't linked to anyone yet — claim it silently
            if (user && !existing.userId) {
              try {
                const claimed = await claimParticipant(calendarId, existing.id);
                resolved = claimed;
                writeStorage(calendarId, {
                  participantId: claimed.id,
                  name: claimed.name,
                  colour: claimed.colour,
                });
              } catch {
                // Claim failed (already linked, or user already joined) — fall through
                // If user already has a participant, the userId match above would have found it
                // on a refetch. For now, just use the localStorage participant as-is.
                resolved = existing;
              }
            } else {
              resolved = existing;
            }
          } else {
            // Stale participant — pre-fill name
            prefillName = stored.name;
          }
        }

        // 4. Pre-fill join form with user name if signed in, then fall back to
        //    the last name this browser used on any calendar.
        if (!resolved && prefillName === null) {
          if (user?.name) {
            prefillName = user.name;
          } else {
            const last = readLastName();
            if (last) prefillName = last;
          }
        }

        if (prefillName !== null) {
          setStoredName(prefillName);
        }

        if (resolved) {
          setCurrentParticipant(resolved);
          participantIdRef.current = resolved.id;
          writeStorage(calendarId, {
            participantId: resolved.id,
            name: resolved.name,
            colour: resolved.colour,
          });

          // Seed selected dates, pruning past dates
          const today = getToday();
          const userDates = data.availability
            .filter((a) => a.participantId === resolved!.id && a.date >= today)
            .map((a) => a.date);
          setSelectedDates(new Set(userDates));

          const seededNotes = new Map<string, string>();
          const seededVotes = new Set<string>();
          for (const a of data.availability) {
            if (a.participantId === resolved!.id && a.date >= today) {
              if (a.note) seededNotes.set(a.date, a.note);
              if (a.preferred) seededVotes.add(a.date);
            }
          }
          setNotesByDate(seededNotes);
          setVotesByDate(seededVotes);
        }
      } catch (err) {
        if (cancelled) return;
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
  }, [calendarId, loadAttempt]);

  // Lightweight refresh used by the live-updates stream. Unlike the full
  // load effect it does not re-run the claim flow or re-seed selectedDates
  // — those belong to initial mount. Local-mutation suppression prevents
  // our own writes from flickering on echo.
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
      const currentId = participantIdRef.current;
      if (currentId) {
        const fresh = data.participants.find((p) => p.id === currentId);
        if (fresh) setCurrentParticipant(fresh);
        // Re-hydrate our own notes from the authoritative response. Own-note
        // edits are still in flight during LOCAL_MUTATION_SUPPRESS_MS, so the
        // suppressRefreshUntilRef guard above protects them.
        const today = getToday();
        const refreshed = new Map<string, string>();
        const refreshedVotes = new Set<string>();
        for (const a of data.availability) {
          if (a.participantId === currentId && a.date >= today) {
            if (a.note) refreshed.set(a.date, a.note);
            if (a.preferred) refreshedVotes.add(a.date);
          }
        }
        setNotesByDate(refreshed);
        setVotesByDate(refreshedVotes);
      }
    } catch {
      // Transient refresh failure; next event or the polling fallback retries.
    }
  }, [calendarId]);

  const { status: streamStatus } = useCalendarEvents(
    calendar ? calendarId : "",
    currentParticipant?.id ?? null,
    refresh,
  );

  // Serialized save logic
  const flushSave = useCallback(async () => {
    savingRef.current = true;
    setSaveError(false);

    while (
      pendingDatesRef.current !== null ||
      pendingNotesRef.current !== null ||
      pendingPreferredRef.current
    ) {
      const dates = pendingDatesRef.current ?? Array.from(selectedDatesRef.current);
      const notes = pendingNotesRef.current;
      const sendPreferred = pendingPreferredRef.current;
      const preferred = sendPreferred
        ? Array.from(votesByDateRef.current).filter((d) => selectedDatesRef.current.has(d))
        : null;
      pendingDatesRef.current = null;
      pendingNotesRef.current = null;
      pendingPreferredRef.current = false;
      const pid = participantIdRef.current;
      if (!pid) break;

      try {
        await setAvailability(calendarId, {
          participantId: pid,
          dates,
          ...(notes !== null ? { notes } : {}),
          ...(preferred !== null ? { preferred } : {}),
        });
        markLocalMutation();
        analytics.capture("availability_marked", { calendar_id: calendarId });
      } catch {
        setSaveError(true);
        savingRef.current = false;
        return;
      }
    }

    savingRef.current = false;
  }, [calendarId, markLocalMutation]);

  const queueSave = useCallback(
    (dates: string[]) => {
      pendingDatesRef.current = dates;
      markLocalMutation();
      if (!savingRef.current) flushSave();
    },
    [flushSave, markLocalMutation],
  );

  // queueNoteSave schedules a note change without shifting selected dates.
  // Coalesces with an in-flight availability save so rapid typing fans out
  // one SSE refresh, not one per keystroke.
  const queueNoteSave = useCallback(
    (date: string, text: string) => {
      const existing = pendingNotesRef.current ?? {};
      existing[date] = text;
      pendingNotesRef.current = existing;
      markLocalMutation();
      if (!savingRef.current) flushSave();
    },
    [flushSave, markLocalMutation],
  );

  const handleToggle = useCallback(
    (date: string) => {
      const prev = selectedDatesRef.current;
      const next = new Set(prev);
      const removed = next.has(date);
      if (removed) next.delete(date);
      else next.add(date);
      selectedDatesRef.current = next;
      setSelectedDates(next);

      if (removed) {
        // Deselecting drops the row in the backend; mirror that in local state.
        // Don't send a `notes` / `preferred` override — the row deletion drops
        // both server-side.
        setNotesByDate((prevMap) => {
          if (!prevMap.has(date)) return prevMap;
          const m = new Map(prevMap);
          m.delete(date);
          return m;
        });
        setVotesByDate((prevSet) => {
          if (!prevSet.has(date)) return prevSet;
          const s = new Set(prevSet);
          s.delete(date);
          return s;
        });
      }

      queueSave(Array.from(next));
    },
    [queueSave],
  );

  const importView = useImportFromGoogle({
    calendarId,
    user,
    participantId: currentParticipant?.id ?? null,
  });
  const { clearSuggestion } = importView;

  const handleToggleWithSuggestions = useCallback(
    (date: string) => {
      clearSuggestion(date);
      handleToggle(date);
    },
    [clearSuggestion, handleToggle],
  );

  const handleOpenDay = useCallback(
    (date: string) => {
      setOpenDayDate(date);
      analytics.capture("day_sheet_opened", {
        calendar_id: calendarId,
        has_notes: (calendar?.availability ?? []).some(
          (a) => a.date === date && a.note && a.note.length > 0,
        ),
      });
    },
    [calendar, calendarId],
  );

  const handleCloseDay = useCallback(() => {
    setOpenDayDate(null);
  }, []);

  const handleSaveNote = useCallback(
    (date: string, text: string) => {
      // Caller passes the already-trimmed text; "" means clear.
      setNotesByDate((prevMap) => {
        const m = new Map(prevMap);
        const before = m.get(date) ?? "";
        if (text === "") {
          if (!m.has(date)) return prevMap;
          m.delete(date);
        } else {
          if (before === text) return prevMap;
          m.set(date, text);
        }
        // Capture analytics *inside* this updater so we read the before/after
        // from the same snapshot the state transition uses.
        if (text === "" && before !== "") {
          analytics.capture("availability_note_cleared", { calendar_id: calendarId });
        } else if (text !== "" && before === "") {
          analytics.capture("availability_note_added", {
            calendar_id: calendarId,
            length: Array.from(text).length,
          });
        } else if (text !== "" && before !== text) {
          analytics.capture("availability_note_edited", {
            calendar_id: calendarId,
            length: Array.from(text).length,
          });
        }
        return m;
      });
      queueNoteSave(date, text);
    },
    [queueNoteSave, calendarId],
  );

  const handleToggleVote = useCallback(
    (date: string, preferred: boolean) => {
      setVotesByDate((prev) => {
        const next = new Set(prev);
        const before = next.has(date);
        if (preferred) next.add(date);
        else next.delete(date);
        if (before === next.has(date)) return prev;
        votesByDateRef.current = next;
        return next;
      });
      pendingPreferredRef.current = true;
      analytics.capture(preferred ? "vote_added" : "vote_removed", {
        calendar_id: calendarId,
      });
      markLocalMutation();
      if (!savingRef.current) flushSave();
    },
    [calendarId, flushSave, markLocalMutation],
  );

  const handleRemoveAvailabilityForDay = useCallback(
    (date: string) => {
      handleToggle(date);
      setOpenDayDate(null);
    },
    [handleToggle],
  );

  // A date is clearable if it's today or later and not individually finalised.
  // (Finalisation hides the toolbar entirely, but the per-cell check is cheap
  // insurance against future multi-finalisation.)
  const isClearable = useCallback(
    (date: string): boolean => {
      if (date < getToday()) return false;
      if (calendar?.calendar.finalisedDate === date) return false;
      return true;
    },
    [calendar],
  );

  const handleClearAvailability = useCallback(() => {
    const prevSelected = new Set(selectedDatesRef.current);
    const prevNotes = new Map(notesByDate);
    const prevVotes = new Set(votesByDateRef.current);
    const cleared: string[] = [];
    let notesCleared = 0;
    const next = new Set<string>();
    for (const d of prevSelected) {
      if (isClearable(d)) {
        cleared.push(d);
        if (prevNotes.has(d)) notesCleared += 1;
      } else {
        next.add(d);
      }
    }
    if (cleared.length === 0) return;

    selectedDatesRef.current = next;
    setSelectedDates(next);
    if (notesCleared > 0) {
      setNotesByDate((prev) => {
        const m = new Map(prev);
        for (const d of cleared) m.delete(d);
        return m;
      });
    }
    setVotesByDate((prev) => {
      let changed = false;
      const s = new Set(prev);
      for (const d of cleared) {
        if (s.delete(d)) changed = true;
      }
      if (changed) votesByDateRef.current = s;
      return changed ? s : prev;
    });
    queueSave(Array.from(next));

    analytics.capture("availability_cleared", {
      calendar_id: calendarId,
      count: cleared.length,
      notes_cleared: notesCleared,
    });

    const undo = () => {
      selectedDatesRef.current = prevSelected;
      setSelectedDates(prevSelected);
      setNotesByDate(prevNotes);
      setVotesByDate(prevVotes);
      votesByDateRef.current = prevVotes;
      queueSave(Array.from(prevSelected));
      analytics.capture("availability_clear_undone", {
        calendar_id: calendarId,
        count: cleared.length,
      });
    };

    const plural = cleared.length === 1 ? "date" : "dates";
    showToast({
      message: `Cleared ${cleared.length} ${plural}.`,
      action: { label: "Undo", onClick: undo },
    });
  }, [notesByDate, isClearable, queueSave, calendarId, showToast]);

  const canClear = useMemo(() => {
    if (selectedDates.size === 0) return false;
    for (const d of selectedDates) {
      if (isClearable(d)) return true;
    }
    return false;
  }, [selectedDates, isClearable]);

  const handleRangeToggle = useCallback(
    (dates: string[], target: boolean) => {
      if (dates.length === 0) return;
      const next = new Set(selectedDatesRef.current);
      if (target) {
        for (const d of dates) next.add(d);
      } else {
        for (const d of dates) next.delete(d);
      }
      selectedDatesRef.current = next;
      setSelectedDates(next);

      if (!target) {
        setNotesByDate((prevMap) => {
          let changed = false;
          const m = new Map(prevMap);
          for (const d of dates) {
            if (m.delete(d)) changed = true;
          }
          return changed ? m : prevMap;
        });
        setVotesByDate((prev) => {
          let changed = false;
          const s = new Set(prev);
          for (const d of dates) {
            if (s.delete(d)) changed = true;
          }
          if (changed) votesByDateRef.current = s;
          return changed ? s : prev;
        });
      }

      queueSave(Array.from(next));
    },
    [queueSave],
  );

  const handleRetry = useCallback(() => {
    queueSave(Array.from(selectedDates));
  }, [selectedDates, queueSave]);

  const handleFinalise = useCallback(
    async (date: string) => {
      markLocalMutation();
      try {
        const updated = await finaliseCalendar(calendarId, date);
        setCalendar((prev) =>
          prev
            ? {
                ...prev,
                calendar: {
                  ...prev.calendar,
                  finalisedDate: updated.finalisedDate,
                  finalisedAt: updated.finalisedAt,
                },
              }
            : prev,
        );
        analytics.capture("calendar_finalised", { calendar_id: calendarId });
      } catch (err) {
        if (err instanceof ApiError && err.status === 409) {
          // Stale client state — refetch
          try {
            const fresh = await getCalendar(calendarId);
            setCalendar(fresh);
          } catch {
            // leave as-is
          }
        }
      }
    },
    [calendarId, markLocalMutation],
  );

  const [renameDraft, setRenameDraft] = useState<string | null>(null);
  const renameInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (renameDraft !== null) {
      renameInputRef.current?.focus();
      renameInputRef.current?.select();
    }
  }, [renameDraft]);

  const handleRenameStart = useCallback(() => {
    setRenameDraft(calendar?.calendar.name ?? "");
  }, [calendar]);

  const handleRenameCancel = useCallback(() => {
    setRenameDraft(null);
  }, []);

  const handleRenameCommit = useCallback(async () => {
    const previous = calendar;
    if (!previous) {
      setRenameDraft(null);
      return;
    }
    const next = (renameInputRef.current?.value ?? "").trim();
    if (next === "" || next === previous.calendar.name) {
      setRenameDraft(null);
      return;
    }

    setRenameDraft(null);
    setCalendar((prev) => (prev ? { ...prev, calendar: { ...prev.calendar, name: next } } : prev));
    markLocalMutation();

    try {
      const updated = await renameCalendar(calendarId, next);
      setCalendar((prev) =>
        prev ? { ...prev, calendar: { ...prev.calendar, name: updated.name } } : prev,
      );
      analytics.capture("calendar_renamed", { calendar_id: calendarId });
    } catch {
      setCalendar(previous);
    }
  }, [calendarId, calendar, markLocalMutation]);

  const handleEnablePassphrase = useCallback(async () => {
    if (!calendar) return;
    markLocalMutation();
    try {
      const passphrase = await rotateCalendarPassphrase(calendarId);
      setCalendar((prev) =>
        prev
          ? {
              ...prev,
              calendar: { ...prev.calendar, hasPassphrase: true, passphrase },
            }
          : prev,
      );
      analytics.capture("calendar_passphrase_set", {
        calendar_id: calendarId,
        action: "enabled",
      });
    } catch {
      // Leave state unchanged; the owner can retry.
    }
  }, [calendar, calendarId, markLocalMutation]);

  const handleRegeneratePassphrase = useCallback(async () => {
    if (!calendar) return;
    setRegeneratePassphraseOpen(true);
  }, [calendar]);

  const confirmRegeneratePassphrase = useCallback(async () => {
    if (!calendar) return;
    setRegeneratePassphraseOpen(false);
    markLocalMutation();
    try {
      const passphrase = await rotateCalendarPassphrase(calendarId);
      setCalendar((prev) =>
        prev
          ? {
              ...prev,
              calendar: { ...prev.calendar, hasPassphrase: true, passphrase },
            }
          : prev,
      );
      analytics.capture("calendar_passphrase_set", {
        calendar_id: calendarId,
        action: "rotated",
      });
    } catch {
      // noop
    }
  }, [calendar, calendarId, markLocalMutation]);

  const handleDisablePassphrase = useCallback(async () => {
    if (!calendar) return;
    setRemovePassphraseOpen(true);
  }, [calendar]);

  const confirmDisablePassphrase = useCallback(async () => {
    if (!calendar) return;
    setRemovePassphraseOpen(false);
    markLocalMutation();
    try {
      await clearCalendarPassphrase(calendarId);
      setCalendar((prev) =>
        prev
          ? {
              ...prev,
              calendar: { ...prev.calendar, hasPassphrase: false, passphrase: undefined },
            }
          : prev,
      );
      analytics.capture("calendar_passphrase_set", {
        calendar_id: calendarId,
        action: "cleared",
      });
    } catch {
      // noop
    }
  }, [calendar, calendarId, markLocalMutation]);

  const handleRemoveParticipant = useCallback(
    (target: Participant) => {
      setParticipantToRemove(target);
    },
    [],
  );

  const confirmRemoveParticipant = useCallback(
    async () => {
      const target = participantToRemove;
      if (!target) return;
      setParticipantToRemove(null);

      const previous = calendar;
      setCalendar((prev) =>
        prev
          ? {
              ...prev,
              participants: prev.participants.filter((p) => p.id !== target.id),
              availability: prev.availability.filter((a) => a.participantId !== target.id),
            }
          : prev,
      );
      markLocalMutation();

      try {
        await removeParticipant(calendarId, target.id);
        analytics.capture("participant_removed", { calendar_id: calendarId });
      } catch {
        setCalendar(previous);
      }
    },
    [participantToRemove, calendarId, calendar, markLocalMutation],
  );

  const handleRenameParticipant = useCallback(
    async (target: Participant, newName: string) => {
      const previous = calendar;
      const callerPid = participantIdRef.current;
      setCalendar((prev) =>
        prev
          ? {
              ...prev,
              participants: prev.participants.map((p) =>
                p.id === target.id ? { ...p, name: newName } : p,
              ),
            }
          : prev,
      );
      if (currentParticipant && currentParticipant.id === target.id) {
        setCurrentParticipant({ ...currentParticipant, name: newName });
        writeStorage(calendarId, {
          participantId: currentParticipant.id,
          name: newName,
          colour: currentParticipant.colour,
        });
      }
      markLocalMutation();
      try {
        await renameParticipant(calendarId, target.id, newName, callerPid);
        analytics.capture("participant_renamed", {
          calendar_id: calendarId,
          self: callerPid === target.id,
        });
      } catch (err) {
        setCalendar(previous);
        if (currentParticipant && currentParticipant.id === target.id) {
          setCurrentParticipant(currentParticipant);
        }
        throw err;
      }
    },
    [calendarId, calendar, currentParticipant, markLocalMutation],
  );

  const handleUnfinalise = useCallback(async () => {
    markLocalMutation();
    try {
      const updated = await unfinaliseCalendar(calendarId);
      setCalendar((prev) =>
        prev
          ? {
              ...prev,
              calendar: {
                ...prev.calendar,
                finalisedDate: updated.finalisedDate,
                finalisedAt: updated.finalisedAt,
              },
            }
          : prev,
      );
      analytics.capture("calendar_unfinalised", { calendar_id: calendarId });
    } catch {
      // ignore; banner stays and user can retry
    }
  }, [calendarId, markLocalMutation]);

  const handleJoin = useCallback(
    async (name: string, email?: string | null) => {
      setJoinError(null);
      markLocalMutation();
      try {
        const participant = await addParticipant(calendarId, { name, email });
        setCurrentParticipant(participant);
        participantIdRef.current = participant.id;
        writeStorage(calendarId, {
          participantId: participant.id,
          name,
          colour: participant.colour,
        });
        writeLastName(name);
        analytics.capture("participant_added", { calendar_id: calendarId });
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
          if (err.code === "calendar_full") {
            setJoinError(`This calendar is full (${MAX_PARTICIPANTS_PER_CALENDAR} people).`);
          } else if (err.code === "already_joined") {
            setJoinError("You've already joined this calendar");
          } else {
            setJoinError("That name is already taken");
          }
        } else {
          setJoinError("Something went wrong");
        }
      }
    },
    [calendarId, markLocalMutation],
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

  // Aggregate vote counts per date across all participants. Current user's
  // votes come from local state (optimistic); other participants from the
  // authoritative fetch.
  const voteCountsByDate = useMemo(() => {
    const map = new Map<string, number>();
    if (!calendar) return map;
    for (const a of calendar.availability) {
      if (currentParticipant && a.participantId === currentParticipant.id) continue;
      if (!a.preferred) continue;
      map.set(a.date, (map.get(a.date) ?? 0) + 1);
    }
    if (currentParticipant) {
      for (const date of votesByDate) {
        if (!selectedDates.has(date)) continue;
        map.set(date, (map.get(date) ?? 0) + 1);
      }
    }
    return map;
  }, [calendar, currentParticipant, votesByDate, selectedDates]);

  // Set of participant IDs who preferred a given date — drives the ★ next to
  // each name in the day sheet.
  const preferredByDate = useMemo(() => {
    const map = new Map<string, Set<string>>();
    if (!calendar) return map;
    for (const a of calendar.availability) {
      if (currentParticipant && a.participantId === currentParticipant.id) continue;
      if (!a.preferred) continue;
      const set = map.get(a.date) ?? new Set<string>();
      set.add(a.participantId);
      map.set(a.date, set);
    }
    if (currentParticipant) {
      for (const date of votesByDate) {
        if (!selectedDates.has(date)) continue;
        const set = map.get(date) ?? new Set<string>();
        set.add(currentParticipant.id);
        map.set(date, set);
      }
    }
    return map;
  }, [calendar, currentParticipant, votesByDate, selectedDates]);

  // noteAuthorsByDate drives the cell's note-dot colour rule: when exactly
  // one participant has a note on a given date, the dot takes that
  // participant's colour; otherwise it's neutral.
  const noteAuthorsByDate = useMemo(() => {
    const map = new Map<string, Set<string>>();
    if (!calendar) return map;
    for (const a of calendar.availability) {
      if (currentParticipant && a.participantId === currentParticipant.id) continue;
      if (!a.note) continue;
      const set = map.get(a.date) ?? new Set<string>();
      set.add(a.participantId);
      map.set(a.date, set);
    }
    if (currentParticipant) {
      for (const [date, text] of notesByDate) {
        if (!text) continue;
        const set = map.get(date) ?? new Set<string>();
        set.add(currentParticipant.id);
        map.set(date, set);
      }
    }
    return map;
  }, [calendar, currentParticipant, notesByDate]);

  if (loading) {
    return (
      <div className={styles.page}>
        <div className={styles.container}>
          <p className={styles.loading}>Loading…</p>
        </div>
      </div>
    );
  }

  if (requiresPassphrase) {
    return (
      <div className={styles.page}>
        <div className={styles.container}>
          <PassphraseGate
            calendarId={calendarId}
            initialError={magicLinkError}
            onUnlocked={() => {
              setMagicLinkError(null);
              setRequiresPassphrase(false);
              retryLoad();
            }}
          />
        </div>
      </div>
    );
  }

  if (error || !calendar) {
    const kind: LoadError = error ?? "not-found";
    const heading = kind === "not-found" ? "Calendar not found" : "Couldn't load this calendar";
    const sub =
      kind === "not-found"
        ? "The link may be wrong or the calendar may have been removed."
        : "Something went wrong on our end. Check your connection and try again.";

    return (
      <div className={styles.page}>
        <div className={styles.errorPanel}>
          <h1 className={styles.errorHeading}>{heading}</h1>
          <p className={styles.errorSub}>{sub}</p>
          {kind === "not-found" ? (
            <Link href="/" className={styles.errorCta}>
              Back to home
            </Link>
          ) : (
            <button type="button" className={styles.errorCta} onClick={retryLoad}>
              Try again
            </button>
          )}
        </div>
      </div>
    );
  }

  const calendarName = calendar.calendar.name || "Untitled Calendar";
  const calendarNamePlain = titlePlain(calendarName);
  const calendarEmoji = calendar.calendar.emoji;
  const finalisedDate = calendar.calendar.finalisedDate ?? null;
  const isOwner = !!user && calendar.calendar.ownerId === user.id;
  const isFull = calendar.participants.length >= MAX_PARTICIPANTS_PER_CALENDAR;

  const participantCount = calendar.participants.length;
  // Distinct dates anyone has marked themselves available (excluding past).
  // Used to drive the second meta pill: weekend-only ranges read as
  // "{n} weekends", anything mixed reads as "{n} days".
  const today = getToday();
  const distinctDates = new Set<string>();
  for (const a of calendar.availability) {
    if (a.date >= today) distinctDates.add(a.date);
  }
  for (const d of selectedDates) {
    if (d >= today) distinctDates.add(d);
  }
  const dateCount = distinctDates.size;
  let allWeekends = dateCount > 0;
  for (const d of distinctDates) {
    const [y, m, day] = d.split("-").map(Number);
    const dow = new Date(y, m - 1, day).getDay();
    if (dow !== 0 && dow !== 6) {
      allWeekends = false;
      break;
    }
  }
  const dateLabel = dateCount === 0
    ? "no dates yet"
    : allWeekends
      ? `${Math.ceil(dateCount / 2)} ${Math.ceil(dateCount / 2) === 1 ? "weekend" : "weekends"}`
      : `${dateCount} ${dateCount === 1 ? "day" : "days"}`;
  const peopleLabel = `${participantCount} ${participantCount === 1 ? "person" : "people"}`;

  return (
    <div className={styles.page}>
      <div className={styles.container}>
        <div className={styles.header}>
          <div className={styles.titleRow}>
            {renameDraft !== null ? (
              <input
                ref={renameInputRef}
                type="text"
                className={styles.headingInput}
                value={renameDraft}
                onChange={(e) => setRenameDraft(e.target.value)}
                onBlur={handleRenameCommit}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleRenameCommit();
                  } else if (e.key === "Escape") {
                    e.preventDefault();
                    handleRenameCancel();
                  }
                }}
                aria-label="Calendar name"
                aria-describedby="calendar-name-help"
                maxLength={MAX_CALENDAR_NAME_LENGTH}
              />
            ) : (
              <h1 className={styles.heading} aria-label={calendarNamePlain}>
                {renderTitle(calendarName)}
              </h1>
            )}
            {isOwner && !finalisedDate && renameDraft === null ? (
              <button
                type="button"
                className={styles.settingsButton}
                onClick={() => setSettingsOpen(true)}
                aria-label="Calendar settings"
              >
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <circle cx="12" cy="12" r="3" />
                  <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
                </svg>
              </button>
            ) : null}
            <span className={styles.liveStatus}>
              <LiveStatusDot status={streamStatus} />
            </span>
          </div>
          {renameDraft !== null ? (
            <p id="calendar-name-help" className={styles.renameHint}>
              Wrap a word in *asterisks* to italicise it.
            </p>
          ) : null}
          <div className={styles.metaRow}>
            <span className="pill">{peopleLabel}</span>
            <span className="pill">{dateLabel}</span>
          </div>
        </div>

        {finalisedDate ? (
          <FinalisedBanner
            date={finalisedDate}
            emoji={calendarEmoji || "📅"}
            calendarId={calendar.calendar.id}
            canUnfinalise={isOwner}
            onUnfinalise={handleUnfinalise}
            participants={calendar.participants}
            availability={calendar.availability}
            voteCount={voteCountsByDate.get(finalisedDate) ?? 0}
          />
        ) : null}

        {!currentParticipant && !finalisedDate ? (
          isFull ? (
            <p className={styles.emptyHint}>
              This calendar is full ({MAX_PARTICIPANTS_PER_CALENDAR} people).
            </p>
          ) : (
            <JoinForm onJoin={handleJoin} error={joinError} defaultName={storedName} hideEmail={!!user} />
          )
        ) : null}

        {!currentParticipant && finalisedDate ? (
          <p className={styles.emptyHint}>
            This calendar is finalised — no further changes needed.
          </p>
        ) : null}

        {saveError ? (
          <div className={styles.banner} role="alert">
            <span>Save failed</span>
            <button type="button" className={styles.retryButton} onClick={handleRetry}>
              Retry
            </button>
          </div>
        ) : null}

        {calendar.participants.length === 0 && !currentParticipant ? (
          <p className={styles.emptyHint}>
            Share the link — when people join, you&apos;ll see their availability here.
          </p>
        ) : null}

        {!finalisedDate ? (
          <BestDay
            participants={calendar.participants}
            freeByDate={freeByDate}
            voteCountsByDate={voteCountsByDate}
          />
        ) : null}

        {!finalisedDate && currentParticipant ? (
          <div>{importView.entry}</div>
        ) : null}
        {importView.chrome}
        {importView.interstitial}
        <CalendarGrid
          participants={calendar.participants}
          freeByDate={freeByDate}
          noteAuthorsByDate={noteAuthorsByDate}
          voteCountsByDate={voteCountsByDate}
          currentParticipantId={currentParticipant?.id ?? null}
          suggestedFreeDates={importView.suggestedFree}
          suggestedBusyDates={importView.suggestedBusy}
          onToggle={handleToggleWithSuggestions}
          onOpenDay={currentParticipant ? handleOpenDay : undefined}
          onRangeToggle={handleRangeToggle}
          onClear={currentParticipant ? handleClearAvailability : undefined}
          canClear={canClear}
          selectedDates={selectedDates}
          joined={!!currentParticipant}
          finalisedDate={finalisedDate}
        />

        {duplicateOpen ? (
          <DuplicateCalendarSheet
            calendarId={calendarId}
            sourceName={calendarName}
            onClose={() => setDuplicateOpen(false)}
            onDuplicated={(newId) => {
              setDuplicateOpen(false);
              router.push(`/calendars/${newId}`);
            }}
          />
        ) : null}

        {openDayDate && currentParticipant ? (
          <DayDetailSheet
            date={openDayDate}
            participants={calendar.participants}
            availability={calendar.availability}
            currentParticipantId={currentParticipant.id}
            currentNote={notesByDate.get(openDayDate) ?? ""}
            isSelected={selectedDates.has(openDayDate)}
            finalised={finalisedDate === openDayDate}
            isPreferred={votesByDate.has(openDayDate)}
            preferredParticipantIds={preferredByDate.get(openDayDate) ?? new Set()}
            onSaveNote={(text) => handleSaveNote(openDayDate, text)}
            onToggleVote={(preferred) => handleToggleVote(openDayDate, preferred)}
            onRemoveAvailability={() => handleRemoveAvailabilityForDay(openDayDate)}
            onClose={handleCloseDay}
          />
        ) : null}

        {finalisedDate ? (
          calendar.participants.length >= 1 ? (
            <>
              <ShareButton
                calendarName={calendarName}
                calendarEmoji={calendarEmoji}
                finalisedDate={finalisedDate}
                passphrase={calendar.calendar.passphrase ?? null}
              />
              <ParticipantList
                participants={calendar.participants}
                currentParticipantId={currentParticipant?.id ?? null}
                canRemove={isOwner}
                canRenameOthers={isOwner}
                onRemove={handleRemoveParticipant}
                onRename={currentParticipant ? handleRenameParticipant : undefined}
              />
            </>
          ) : null
        ) : calendar.participants.length >= 2 ? (
          <>
            <div className={styles.summaryCard}>
              <SummaryView
                participants={calendar.participants}
                freeByDate={freeByDate}
                canFinalise={isOwner}
                onFinalise={handleFinalise}
              />
              <ShareButton
                calendarName={calendarName}
                calendarEmoji={calendarEmoji}
                finalisedDate={finalisedDate}
                passphrase={calendar.calendar.passphrase ?? null}
                variant="hero"
              />
            </div>
            <ParticipantList
              participants={calendar.participants}
              currentParticipantId={currentParticipant?.id ?? null}
              canRemove={isOwner}
              canRenameOthers={isOwner}
              onRemove={handleRemoveParticipant}
              onRename={currentParticipant ? handleRenameParticipant : undefined}
            />
          </>
        ) : (
          <>
            <InvitePrompt
              calendarName={calendarName}
              calendarEmoji={calendarEmoji}
              finalisedDate={finalisedDate}
              passphrase={calendar.calendar.passphrase ?? null}
            />
            {calendar.participants.length === 1 ? (
              <ParticipantList
                participants={calendar.participants}
                currentParticipantId={currentParticipant?.id ?? null}
                canRemove={isOwner}
                canRenameOthers={isOwner}
                onRemove={handleRemoveParticipant}
                onRename={currentParticipant ? handleRenameParticipant : undefined}
              />
            ) : null}
          </>
        )}

        {isOwner && !finalisedDate ? (
          <WhosMissingPanel
            calendarId={calendar.calendar.id}
            participants={calendar.participants}
          />
        ) : null}

        {removePassphraseOpen ? (
          <ConfirmDialog
            title="Remove passphrase?"
            message="The calendar will be openable without it. You can set a new one later."
            confirmLabel="Remove passphrase"
            cancelLabel="Keep it"
            destructive
            onConfirm={confirmDisablePassphrase}
            onCancel={() => setRemovePassphraseOpen(false)}
          />
        ) : null}

        {regeneratePassphraseOpen ? (
          <ConfirmDialog
            title="Regenerate passphrase?"
            message="The old passphrase stops working immediately. Anyone using the old link will need the new one."
            confirmLabel="Regenerate"
            cancelLabel="Cancel"
            destructive
            onConfirm={confirmRegeneratePassphrase}
            onCancel={() => setRegeneratePassphraseOpen(false)}
          />
        ) : null}

        {participantToRemove ? (
          <ConfirmDialog
            title={`Remove ${participantToRemove.name}?`}
            message="Their availability will be cleared from the calendar."
            confirmLabel="Remove"
            cancelLabel="Cancel"
            destructive
            onConfirm={confirmRemoveParticipant}
            onCancel={() => setParticipantToRemove(null)}
          />
        ) : null}

        {settingsOpen ? (
          <CalendarSettingsSheet
            hasPassphrase={calendar.calendar.hasPassphrase ?? false}
            passphrase={calendar.calendar.passphrase ?? null}
            onRename={() => {
              setSettingsOpen(false);
              handleRenameStart();
            }}
            onEnablePassphrase={handleEnablePassphrase}
            onRegeneratePassphrase={handleRegeneratePassphrase}
            onDisablePassphrase={handleDisablePassphrase}
            onDuplicate={() => {
              setSettingsOpen(false);
              setDuplicateOpen(true);
            }}
            onClose={() => setSettingsOpen(false)}
          />
        ) : null}
      </div>
    </div>
  );
}
