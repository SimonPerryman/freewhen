"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { MutableRefObject } from "react";
import { setAvailability } from "./api";
import analytics from "./analytics";
import { localToday } from "./localDate";
import { useToast } from "./toast";
import type { CalendarWithDetails } from "./types";

export interface UseAvailabilitySaveOptions {
  calendarId: string;
  // Who we save as. A ref because the save loop is async and must see the
  // participant id at send time (e.g. right after joining).
  participantIdRef: MutableRefObject<string | null>;
  // The calendar's finalised date, if any — never cleared by "Clear".
  finalisedDate: string | null;
  markLocalMutation: () => void;
}

export interface UseAvailabilitySaveResult {
  selectedDates: Set<string>;
  // The current participant's own notes keyed by ISO date.
  notesByDate: Map<string, string>;
  // The current participant's own preferred-date set.
  votesByDate: Set<string>;
  saveError: boolean;
  canClear: boolean;
  // Seed from a full load: selected dates, notes and votes (past dates pruned).
  seed: (data: CalendarWithDetails, participantId: string) => void;
  // Re-hydrate notes and votes from a live refresh (selection is left alone).
  rehydrate: (data: CalendarWithDetails, participantId: string) => void;
  handleToggle: (date: string) => void;
  handleRangeToggle: (dates: string[], target: boolean) => void;
  handleSaveNote: (date: string, text: string) => void;
  handleToggleVote: (date: string, preferred: boolean) => void;
  handleClearAvailability: () => void;
  handleRetry: () => void;
}

function ownNotesAndVotes(
  data: CalendarWithDetails,
  participantId: string,
  today: string,
): { notes: Map<string, string>; votes: Set<string> } {
  const notes = new Map<string, string>();
  const votes = new Set<string>();
  for (const a of data.availability) {
    if (a.participantId === participantId && a.date >= today) {
      if (a.note) notes.set(a.date, a.note);
      if (a.preferred) votes.add(a.date);
    }
  }
  return { notes, votes };
}

/**
 * The current participant's own availability: optimistic local state plus a
 * serialized save queue. At most one PUT is in flight; edits made while it
 * runs coalesce into the next one. Notes and votes are only sent when they
 * changed (absent fields hit the backend's "don't touch" path).
 */
export function useAvailabilitySave({
  calendarId,
  participantIdRef,
  finalisedDate,
  markLocalMutation,
}: UseAvailabilitySaveOptions): UseAvailabilitySaveResult {
  const { showToast } = useToast();
  const [selectedDates, setSelectedDates] = useState<Set<string>>(new Set());
  const [notesByDate, setNotesByDate] = useState<Map<string, string>>(new Map());
  const [votesByDate, setVotesByDate] = useState<Set<string>>(new Set());
  const [saveError, setSaveError] = useState(false);

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
  // Mirrors selectedDates so handlers can compute the next set outside of a
  // setState updater — the updater must stay pure (StrictMode invokes it twice).
  const selectedDatesRef = useRef<Set<string>>(selectedDates);
  useEffect(() => {
    selectedDatesRef.current = selectedDates;
  }, [selectedDates]);

  const seed = useCallback((data: CalendarWithDetails, participantId: string) => {
    // Seed selected dates, pruning past dates
    const today = localToday();
    const userDates = data.availability
      .filter((a) => a.participantId === participantId && a.date >= today)
      .map((a) => a.date);
    setSelectedDates(new Set(userDates));
    const { notes, votes } = ownNotesAndVotes(data, participantId, today);
    setNotesByDate(notes);
    setVotesByDate(votes);
  }, []);

  const rehydrate = useCallback((data: CalendarWithDetails, participantId: string) => {
    // Re-hydrate our own notes from the authoritative response. Own-note
    // edits are still in flight during the local-mutation window, so the
    // refresh suppression in useCalendarData protects them.
    const { notes, votes } = ownNotesAndVotes(data, participantId, localToday());
    setNotesByDate(notes);
    setVotesByDate(votes);
  }, []);

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
  }, [calendarId, markLocalMutation, participantIdRef]);

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

  // A date is clearable if it's today or later and not individually finalised.
  // (Finalisation hides the toolbar entirely, but the per-cell check is cheap
  // insurance against future multi-finalisation.)
  const isClearable = useCallback(
    (date: string): boolean => {
      if (date < localToday()) return false;
      if (finalisedDate === date) return false;
      return true;
    },
    [finalisedDate],
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

  return {
    selectedDates,
    notesByDate,
    votesByDate,
    saveError,
    canClear,
    seed,
    rehydrate,
    handleToggle,
    handleRangeToggle,
    handleSaveNote,
    handleToggleVote,
    handleClearAvailability,
    handleRetry,
  };
}
