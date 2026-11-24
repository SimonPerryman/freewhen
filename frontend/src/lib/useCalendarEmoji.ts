"use client";

import { useCallback } from "react";
import type { Dispatch, SetStateAction } from "react";
import { updateCalendar } from "./api";
import analytics from "./analytics";
import { useToast } from "./toast";
import type { CalendarWithDetails } from "./types";

export interface UseCalendarEmojiOptions {
  calendarId: string;
  calendar: CalendarWithDetails | null;
  setCalendar: Dispatch<SetStateAction<CalendarWithDetails | null>>;
  markLocalMutation: () => void;
}

/**
 * Owner-chosen calendar emoji: optimistic update, rollback if the PATCH
 * fails, and an Undo toast that puts the previous emoji back.
 */
export function useCalendarEmoji({
  calendarId,
  calendar,
  setCalendar,
  markLocalMutation,
}: UseCalendarEmojiOptions): (emoji: string) => Promise<void> {
  const { showToast } = useToast();

  const apply = useCallback(
    async (emoji: string, previous: string | undefined): Promise<boolean> => {
      const set = (value: string | undefined) =>
        setCalendar((prev) =>
          prev ? { ...prev, calendar: { ...prev.calendar, emoji: value } } : prev,
        );
      set(emoji);
      markLocalMutation();
      try {
        const updated = await updateCalendar(calendarId, { emoji });
        set(updated.emoji);
        return true;
      } catch {
        set(previous);
        showToast({ message: "Couldn't change the emoji. Try again." });
        return false;
      }
    },
    [calendarId, markLocalMutation, setCalendar, showToast],
  );

  return useCallback(
    async (emoji: string) => {
      const previous = calendar?.calendar.emoji;
      if (!calendar || emoji === previous) return;
      if (!(await apply(emoji, previous))) return;
      analytics.capture("calendar_emoji_changed", { calendar_id: calendarId });
      // Undo only makes sense when there was an emoji to go back to.
      showToast({
        message: `Emoji changed to ${emoji}`,
        action: previous
          ? { label: "Undo", onClick: () => void apply(previous, emoji) }
          : undefined,
      });
    },
    [apply, calendar, calendarId, showToast],
  );
}
