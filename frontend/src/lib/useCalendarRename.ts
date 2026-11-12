"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { Dispatch, MutableRefObject, SetStateAction } from "react";
import { renameCalendar } from "./api";
import analytics from "./analytics";
import type { CalendarWithDetails } from "./types";

export interface UseCalendarRenameOptions {
  calendarId: string;
  calendar: CalendarWithDetails | null;
  setCalendar: Dispatch<SetStateAction<CalendarWithDetails | null>>;
  markLocalMutation: () => void;
}

export interface UseCalendarRenameResult {
  // null when not renaming; otherwise the input's current value.
  renameDraft: string | null;
  setRenameDraft: Dispatch<SetStateAction<string | null>>;
  renameInputRef: MutableRefObject<HTMLInputElement | null>;
  handleRenameStart: () => void;
  handleRenameCancel: () => void;
  handleRenameCommit: () => Promise<void>;
}

/**
 * Inline calendar rename (owner only): draft state, focus-on-open, and an
 * optimistic commit that rolls back if the PATCH fails.
 */
export function useCalendarRename({
  calendarId,
  calendar,
  setCalendar,
  markLocalMutation,
}: UseCalendarRenameOptions): UseCalendarRenameResult {
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
  }, [calendarId, calendar, markLocalMutation, setCalendar]);

  return {
    renameDraft,
    setRenameDraft,
    renameInputRef,
    handleRenameStart,
    handleRenameCancel,
    handleRenameCommit,
  };
}
