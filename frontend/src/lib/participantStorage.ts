// Page-level memory of "who am I on this calendar", kept in localStorage:
//
// - `freewhen:<calendarId>` — the participant this browser joined as, so a
//   reload (or a later visit) skips the join form.
// - `freewhen:lastName` — the last name used on any calendar, to prefill the
//   join form on the next one.
//
// The per-participant *secret* lives separately in participantToken.ts: it is
// owned by the API client (sent on every calendar-scoped request) and is
// stored under its own key so rewriting this record never drops it.

import { MAX_PARTICIPANT_NAME_LENGTH } from "./limits";

const STORAGE_PREFIX = "freewhen:";
const LAST_NAME_KEY = "freewhen:lastName";

export interface StoredParticipant {
  participantId: string;
  name: string;
  colour?: string;
}

export function readStoredParticipant(calendarId: string): StoredParticipant | null {
  try {
    const raw = localStorage.getItem(`${STORAGE_PREFIX}${calendarId}`);
    if (!raw) return null;
    return JSON.parse(raw) as StoredParticipant;
  } catch {
    return null;
  }
}

// Deliberately unguarded: callers run inside try blocks that surface a
// storage failure (e.g. the join flow reports "Something went wrong").
export function writeStoredParticipant(calendarId: string, data: StoredParticipant): void {
  localStorage.setItem(`${STORAGE_PREFIX}${calendarId}`, JSON.stringify(data));
}

export function readLastName(): string {
  try {
    const raw = localStorage.getItem(LAST_NAME_KEY);
    if (!raw) return "";
    return raw.slice(0, MAX_PARTICIPANT_NAME_LENGTH);
  } catch {
    return "";
  }
}

export function writeLastName(name: string): void {
  try {
    localStorage.setItem(LAST_NAME_KEY, name);
  } catch {
    // ignore quota / disabled storage
  }
}
