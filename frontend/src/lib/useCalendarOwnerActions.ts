"use client";

import { useCallback, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
import {
  clearCalendarPassphrase,
  finaliseCalendar,
  getCalendar,
  removeParticipant,
  rotateCalendarPassphrase,
  unfinaliseCalendar,
  ApiError,
} from "./api";
import analytics from "./analytics";
import type { CalendarWithDetails, Participant } from "./types";

export interface UseCalendarOwnerActionsOptions {
  calendarId: string;
  calendar: CalendarWithDetails | null;
  setCalendar: Dispatch<SetStateAction<CalendarWithDetails | null>>;
  markLocalMutation: () => void;
}

export interface UseCalendarOwnerActionsResult {
  handleFinalise: (date: string) => Promise<void>;
  handleUnfinalise: () => Promise<void>;

  handleEnablePassphrase: () => Promise<void>;
  // Open the confirm dialog; the confirm* handler does the work.
  handleRegeneratePassphrase: () => Promise<void>;
  regeneratePassphraseOpen: boolean;
  confirmRegeneratePassphrase: () => Promise<void>;
  cancelRegeneratePassphrase: () => void;
  handleDisablePassphrase: () => Promise<void>;
  removePassphraseOpen: boolean;
  confirmDisablePassphrase: () => Promise<void>;
  cancelDisablePassphrase: () => void;

  handleRemoveParticipant: (target: Participant) => void;
  participantToRemove: Participant | null;
  confirmRemoveParticipant: () => Promise<void>;
  cancelRemoveParticipant: () => void;
}

/**
 * Owner-only calendar mutations and the confirm-dialog state they need:
 * finalise / unfinalise, passphrase enable / rotate / clear, and removing a
 * participant. All are optimistic or patch local state from the response.
 */
export function useCalendarOwnerActions({
  calendarId,
  calendar,
  setCalendar,
  markLocalMutation,
}: UseCalendarOwnerActionsOptions): UseCalendarOwnerActionsResult {
  const [removePassphraseOpen, setRemovePassphraseOpen] = useState(false);
  const [regeneratePassphraseOpen, setRegeneratePassphraseOpen] = useState(false);
  const [participantToRemove, setParticipantToRemove] = useState<Participant | null>(null);

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
    [calendarId, markLocalMutation, setCalendar],
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
  }, [calendarId, markLocalMutation, setCalendar]);

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
  }, [calendar, calendarId, markLocalMutation, setCalendar]);

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
  }, [calendar, calendarId, markLocalMutation, setCalendar]);

  const cancelRegeneratePassphrase = useCallback(() => {
    setRegeneratePassphraseOpen(false);
  }, []);

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
  }, [calendar, calendarId, markLocalMutation, setCalendar]);

  const cancelDisablePassphrase = useCallback(() => {
    setRemovePassphraseOpen(false);
  }, []);

  const handleRemoveParticipant = useCallback((target: Participant) => {
    setParticipantToRemove(target);
  }, []);

  const confirmRemoveParticipant = useCallback(async () => {
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
  }, [participantToRemove, calendarId, calendar, markLocalMutation, setCalendar]);

  const cancelRemoveParticipant = useCallback(() => {
    setParticipantToRemove(null);
  }, []);

  return {
    handleFinalise,
    handleUnfinalise,
    handleEnablePassphrase,
    handleRegeneratePassphrase,
    regeneratePassphraseOpen,
    confirmRegeneratePassphrase,
    cancelRegeneratePassphrase,
    handleDisablePassphrase,
    removePassphraseOpen,
    confirmDisablePassphrase,
    cancelDisablePassphrase,
    handleRemoveParticipant,
    participantToRemove,
    confirmRemoveParticipant,
    cancelRemoveParticipant,
  };
}
