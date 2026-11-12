"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { MutableRefObject } from "react";
import { setAvailability } from "./api";
import analytics from "./analytics";
import { localToday } from "./localDate";
import { useToast } from "./toast";
import type { CalendarWithDetails, SetAvailabilityRequest } from "./types";

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
  // Re-hydrate notes and votes from a live refresh. The selection is synced
  // too, but only until the first local edit after a seed.
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

  // These refs mirror selectedDates / notesByDate / votesByDate and are the
  // source of truth for handlers: each handler computes the next value from
  // the ref synchronously, writes ref + state with that exact value (via the
  // commit* helpers), then saves. Never put side effects (ref writes, saves,
  // analytics) inside a setState updater — React may run it later during
  // render, so a save sent right after would read the previous value, and
  // StrictMode runs it twice.
  const selectedDatesRef = useRef<Set<string>>(selectedDates);
  const notesByDateRef = useRef<Map<string, string>>(notesByDate);
  const votesByDateRef = useRef<Set<string>>(votesByDate);

  const commitSelected = useCallback((next: Set<string>) => {
    selectedDatesRef.current = next;
    setSelectedDates(next);
  }, []);
  const commitNotes = useCallback((next: Map<string, string>) => {
    notesByDateRef.current = next;
    setNotesByDate(next);
  }, []);
  const commitVotes = useCallback((next: Set<string>) => {
    votesByDateRef.current = next;
    setVotesByDate(next);
  }, []);

  // False from a seed until the first local edit. While false the selection
  // on screen is just what the server sent, so a live refresh may replace it.
  const editedSinceSeedRef = useRef(false);

  const seed = useCallback(
    (data: CalendarWithDetails, participantId: string) => {
      // Seed selected dates, pruning past dates
      const today = localToday();
      const userDates = data.availability
        .filter((a) => a.participantId === participantId && a.date >= today)
        .map((a) => a.date);
      commitSelected(new Set(userDates));
      editedSinceSeedRef.current = false;
      const { notes, votes } = ownNotesAndVotes(data, participantId, today);
      commitNotes(notes);
      commitVotes(votes);
    },
    [commitSelected, commitNotes, commitVotes],
  );

  const rehydrate = useCallback(
    (data: CalendarWithDetails, participantId: string) => {
      // Re-hydrate our own notes from the authoritative response. Own-note
      // edits are still in flight during the local-mutation window, so the
      // refresh suppression in useCalendarData protects them.
      const today = localToday();
      const { notes, votes } = ownNotesAndVotes(data, participantId, today);
      commitNotes(notes);
      commitVotes(votes);
      // The selection is normally left alone (local edits win), but until
      // this page has edited anything it's just a server snapshot. Sync it:
      // after a reload, the seed can predate the save flushed on the way out
      // (see flushOnHide), and a stale selection would be re-sent — and
      // overwrite that save — on the next toggle.
      if (!editedSinceSeedRef.current) {
        commitSelected(
          new Set(
            data.availability
              .filter((a) => a.participantId === participantId && a.date >= today)
              .map((a) => a.date),
          ),
        );
      }
    },
    [commitNotes, commitVotes, commitSelected],
  );

  // Drop local notes and votes for dates that were just deselected. The
  // backend drops the row (and both with it), so no override is sent.
  const pruneNotesAndVotes = useCallback(
    (dates: Iterable<string>) => {
      const notes = new Map(notesByDateRef.current);
      const votes = new Set(votesByDateRef.current);
      let notesChanged = false;
      let votesChanged = false;
      for (const d of dates) {
        if (notes.delete(d)) notesChanged = true;
        if (votes.delete(d)) votesChanged = true;
      }
      if (notesChanged) commitNotes(notes);
      if (votesChanged) commitVotes(votes);
    },
    [commitNotes, commitVotes],
  );

  // Builds the request for whatever is queued but not yet sent, and consumes
  // the queue. Null when nothing is queued or there's no participant yet (the
  // queue is then left alone for a later save).
  const takePending = useCallback((): SetAvailabilityRequest | null => {
    if (
      pendingDatesRef.current === null &&
      pendingNotesRef.current === null &&
      !pendingPreferredRef.current
    ) {
      return null;
    }
    const pid = participantIdRef.current;
    if (!pid) return null;
    const dates = pendingDatesRef.current ?? Array.from(selectedDatesRef.current);
    const notes = pendingNotesRef.current;
    const preferred = pendingPreferredRef.current
      ? Array.from(votesByDateRef.current).filter((d) => selectedDatesRef.current.has(d))
      : null;
    pendingDatesRef.current = null;
    pendingNotesRef.current = null;
    pendingPreferredRef.current = false;
    return {
      participantId: pid,
      dates,
      ...(notes !== null ? { notes } : {}),
      ...(preferred !== null ? { preferred } : {}),
    };
  }, [participantIdRef]);

  // Serialized save logic. Every save is sent with fetch `keepalive` so a
  // save that is in flight when the page unloads isn't cancelled (a
  // cancelled request can cancel the server-side write with it).
  const flushSave = useCallback(async () => {
    savingRef.current = true;
    setSaveError(false);

    for (let req = takePending(); req !== null; req = takePending()) {
      try {
        await setAvailability(calendarId, req, { keepalive: true });
        markLocalMutation();
        analytics.capture("availability_marked", { calendar_id: calendarId });
      } catch {
        setSaveError(true);
        savingRef.current = false;
        return;
      }
    }

    savingRef.current = false;
  }, [calendarId, markLocalMutation, takePending]);

  // Flush queued edits when the page is hidden or unloaded (reload, close,
  // tab switch, app backgrounded on mobile). Only the queued-but-unsent state
  // is sent: the in-flight save (itself keepalive) already carries everything
  // before it, so nothing is sent twice. fetch keepalive, not sendBeacon:
  // the request needs the participant token header, credentials and a JSON
  // content type.
  const flushOnHide = useCallback(() => {
    if (!savingRef.current) {
      // Nothing in flight (e.g. edits queued before joining finished): run
      // the normal loop now. Its fetch is dispatched synchronously, before
      // the page goes away.
      if (
        pendingDatesRef.current !== null ||
        pendingNotesRef.current !== null ||
        pendingPreferredRef.current
      ) {
        void flushSave();
      }
      return;
    }
    // A save is in flight and the loop would only send the queued edits
    // after it returns — too late if the page is unloading. Send them now.
    const req = takePending();
    if (!req) return;
    setAvailability(calendarId, req, { keepalive: true }).then(
      () => markLocalMutation(),
      () => {
        // Still alive (e.g. tab hidden, not closed): re-queue so a retry
        // sends it, and surface the error like a normal failed save.
        if (req.notes) pendingNotesRef.current = { ...req.notes, ...pendingNotesRef.current };
        if (req.preferred) pendingPreferredRef.current = true;
        pendingDatesRef.current ??= Array.from(selectedDatesRef.current);
        setSaveError(true);
      },
    );
  }, [calendarId, flushSave, markLocalMutation, takePending]);

  useEffect(() => {
    const onVisibility = () => {
      if (document.visibilityState === "hidden") flushOnHide();
    };
    window.addEventListener("pagehide", flushOnHide);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      window.removeEventListener("pagehide", flushOnHide);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [flushOnHide]);

  const queueSave = useCallback(
    (dates: string[]) => {
      pendingDatesRef.current = dates;
      editedSinceSeedRef.current = true;
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
      editedSinceSeedRef.current = true;
      markLocalMutation();
      if (!savingRef.current) flushSave();
    },
    [flushSave, markLocalMutation],
  );

  const handleToggle = useCallback(
    (date: string) => {
      const next = new Set(selectedDatesRef.current);
      const removed = next.has(date);
      if (removed) next.delete(date);
      else next.add(date);
      commitSelected(next);

      // Deselecting drops the row in the backend; mirror that in local state.
      // Don't send a `notes` / `preferred` override — the row deletion drops
      // both server-side.
      if (removed) pruneNotesAndVotes([date]);

      queueSave(Array.from(next));
    },
    [commitSelected, pruneNotesAndVotes, queueSave],
  );

  const handleSaveNote = useCallback(
    (date: string, text: string) => {
      // Caller passes the already-trimmed text; "" means clear.
      const prev = notesByDateRef.current;
      const before = prev.get(date) ?? "";
      const changed = text === "" ? prev.has(date) : before !== text;
      if (changed) {
        const next = new Map(prev);
        if (text === "") next.delete(date);
        else next.set(date, text);
        commitNotes(next);
        if (text === "") {
          analytics.capture("availability_note_cleared", { calendar_id: calendarId });
        } else if (before === "") {
          analytics.capture("availability_note_added", {
            calendar_id: calendarId,
            length: Array.from(text).length,
          });
        } else {
          analytics.capture("availability_note_edited", {
            calendar_id: calendarId,
            length: Array.from(text).length,
          });
        }
      }
      queueNoteSave(date, text);
    },
    [commitNotes, queueNoteSave, calendarId],
  );

  const handleToggleVote = useCallback(
    (date: string, preferred: boolean) => {
      const prev = votesByDateRef.current;
      if (prev.has(date) !== preferred) {
        const next = new Set(prev);
        if (preferred) next.add(date);
        else next.delete(date);
        commitVotes(next);
      }
      pendingPreferredRef.current = true;
      editedSinceSeedRef.current = true;
      analytics.capture(preferred ? "vote_added" : "vote_removed", {
        calendar_id: calendarId,
      });
      markLocalMutation();
      if (!savingRef.current) flushSave();
    },
    [calendarId, commitVotes, flushSave, markLocalMutation],
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
    const prevSelected = selectedDatesRef.current;
    const prevNotes = notesByDateRef.current;
    const prevVotes = votesByDateRef.current;
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

    commitSelected(next);
    pruneNotesAndVotes(cleared);
    queueSave(Array.from(next));

    analytics.capture("availability_cleared", {
      calendar_id: calendarId,
      count: cleared.length,
      notes_cleared: notesCleared,
    });

    const undo = () => {
      commitSelected(prevSelected);
      commitNotes(prevNotes);
      commitVotes(prevVotes);
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
  }, [
    isClearable,
    commitSelected,
    commitNotes,
    commitVotes,
    pruneNotesAndVotes,
    queueSave,
    calendarId,
    showToast,
  ]);

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
      commitSelected(next);
      if (!target) pruneNotesAndVotes(dates);
      queueSave(Array.from(next));
    },
    [commitSelected, pruneNotesAndVotes, queueSave],
  );

  const handleRetry = useCallback(() => {
    queueSave(Array.from(selectedDatesRef.current));
  }, [queueSave]);

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
