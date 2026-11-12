"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { Dispatch, MutableRefObject, SetStateAction } from "react";
import { addParticipant, claimParticipant, renameParticipant, ApiError } from "./api";
import analytics from "./analytics";
import { MAX_PARTICIPANTS_PER_CALENDAR } from "./limits";
import {
  readLastName,
  readStoredParticipant,
  writeLastName,
  writeStoredParticipant,
} from "./participantStorage";
import type { CalendarWithDetails, Participant, User } from "./types";

export interface UseParticipantIdentityOptions {
  calendarId: string;
  user: User | null;
  // Current calendar snapshot — the rollback point for optimistic renames.
  calendar: CalendarWithDetails | null;
  setCalendar: Dispatch<SetStateAction<CalendarWithDetails | null>>;
  markLocalMutation: () => void;
}

export interface UseParticipantIdentityResult {
  // The participant this browser / user is on this calendar, if any.
  currentParticipant: Participant | null;
  // Mirrors currentParticipant?.id for async code (the save queue).
  participantIdRef: MutableRefObject<string | null>;
  // False until the first resolveFromLoad finishes. Server-rendered pages
  // use it to hold back "you haven't joined" UI until we actually know.
  identityResolved: boolean;
  // Join-form prefill: stale stored name, signed-in name, or last-used name.
  storedName: string;
  joinError: string | null;
  // Resolve "who am I" after a full load. Returns the resolved participant
  // (so the caller can seed that participant's availability) or null.
  resolveFromLoad: (data: CalendarWithDetails) => Promise<Participant | null>;
  // Pick up server-side changes to our own participant after a live refresh.
  // Returns our participant id, or null when we haven't joined.
  syncFromRefresh: (data: CalendarWithDetails) => string | null;
  handleJoin: (name: string, email?: string | null) => Promise<void>;
  handleRenameParticipant: (target: Participant, newName: string) => Promise<void>;
}

/**
 * Tracks which participant "you" are on a calendar: resolution on load
 * (signed-in match → stored participant → silent claim → name prefill),
 * joining, and keeping the stored record in step with renames.
 */
export function useParticipantIdentity({
  calendarId,
  user,
  calendar,
  setCalendar,
  markLocalMutation,
}: UseParticipantIdentityOptions): UseParticipantIdentityResult {
  const [currentParticipant, setCurrentParticipant] = useState<Participant | null>(null);
  const [storedName, setStoredName] = useState("");
  const [joinError, setJoinError] = useState<string | null>(null);
  const [identityResolved, setIdentityResolved] = useState(false);
  const participantIdRef = useRef<string | null>(null);

  useEffect(() => {
    participantIdRef.current = currentParticipant?.id ?? null;
  }, [currentParticipant]);

  const resolveFromLoad = useCallback(
    async (data: CalendarWithDetails): Promise<Participant | null> => {
      const stored = readStoredParticipant(calendarId);

      // Resolve participant: user_id match first, then localStorage
      let resolved: Participant | undefined;
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
              writeStoredParticipant(calendarId, {
                participantId: claimed.id,
                name: claimed.name,
                colour: claimed.colour,
              });
            } catch {
              // Claim failed (already linked, or user already joined) — fall
              // through and use the localStorage participant as-is.
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
      setIdentityResolved(true);

      if (!resolved) return null;

      setCurrentParticipant(resolved);
      participantIdRef.current = resolved.id;
      writeStoredParticipant(calendarId, {
        participantId: resolved.id,
        name: resolved.name,
        colour: resolved.colour,
      });
      return resolved;
    },
    [calendarId, user],
  );

  const syncFromRefresh = useCallback((data: CalendarWithDetails): string | null => {
    const currentId = participantIdRef.current;
    if (!currentId) return null;
    const fresh = data.participants.find((p) => p.id === currentId);
    if (fresh) setCurrentParticipant(fresh);
    return currentId;
  }, []);

  const handleJoin = useCallback(
    async (name: string, email?: string | null) => {
      setJoinError(null);
      markLocalMutation();
      try {
        const participant = await addParticipant(calendarId, { name, email });
        setCurrentParticipant(participant);
        participantIdRef.current = participant.id;
        writeStoredParticipant(calendarId, {
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
    [calendarId, markLocalMutation, setCalendar],
  );

  // Rename any participant (owner) or yourself. Optimistic; rolls back and
  // rethrows on failure so the row editor can show the error.
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
        writeStoredParticipant(calendarId, {
          participantId: currentParticipant.id,
          name: newName,
          colour: currentParticipant.colour,
        });
      }
      markLocalMutation();
      try {
        await renameParticipant(calendarId, target.id, newName);
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
    [calendarId, calendar, currentParticipant, markLocalMutation, setCalendar],
  );

  return {
    currentParticipant,
    identityResolved,
    participantIdRef,
    storedName,
    joinError,
    resolveFromLoad,
    syncFromRefresh,
    handleJoin,
    handleRenameParticipant,
  };
}
