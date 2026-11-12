import { useMemo } from "react";
import type { CalendarWithDetails, Participant } from "./types";

// Derived per-date views of a calendar's availability. Everyone else's rows
// come from the authoritative fetch; the current participant's (`me`) come
// from local, optimistic state so their edits show before the save lands.

// For each date, the set of participant IDs that are free.
export function buildFreeByDate(
  calendar: CalendarWithDetails | null,
  me: Participant | null,
  selectedDates: Set<string>,
): Map<string, Set<string>> {
  const map = new Map<string, Set<string>>();
  if (!calendar) return map;

  for (const a of calendar.availability) {
    // Skip current user's backend availability — we use selectedDates instead
    if (me && a.participantId === me.id) continue;
    const set = map.get(a.date) ?? new Set<string>();
    set.add(a.participantId);
    map.set(a.date, set);
  }

  // Layer in current user's local selection
  if (me) {
    for (const date of selectedDates) {
      const set = map.get(date) ?? new Set<string>();
      set.add(me.id);
      map.set(date, set);
    }
  }

  return map;
}

// Participants who have responded, i.e. marked at least one date free.
// Joining alone doesn't count — a calendar's owner is a participant from the
// moment it's created, before they've marked anything.
export function respondedParticipantIds(
  freeByDate: ReadonlyMap<string, ReadonlySet<string>>,
): Set<string> {
  const ids = new Set<string>();
  for (const set of freeByDate.values()) {
    for (const id of set) ids.add(id);
  }
  return ids;
}

// How many of `participants` have at least one availability row. Rows from
// participants no longer on the calendar are ignored.
export function countResponded(
  participants: readonly Pick<Participant, "id">[],
  availability: readonly { participantId: string }[],
): number {
  const ids = new Set(availability.map((a) => a.participantId));
  return participants.filter((p) => ids.has(p.id)).length;
}

// Aggregate vote counts per date across all participants.
export function buildVoteCountsByDate(
  calendar: CalendarWithDetails | null,
  me: Participant | null,
  votesByDate: Set<string>,
  selectedDates: Set<string>,
): Map<string, number> {
  const map = new Map<string, number>();
  if (!calendar) return map;
  for (const a of calendar.availability) {
    if (me && a.participantId === me.id) continue;
    if (!a.preferred) continue;
    map.set(a.date, (map.get(a.date) ?? 0) + 1);
  }
  if (me) {
    for (const date of votesByDate) {
      if (!selectedDates.has(date)) continue;
      map.set(date, (map.get(date) ?? 0) + 1);
    }
  }
  return map;
}

// Set of participant IDs who preferred a given date — drives the ★ next to
// each name in the day sheet.
export function buildPreferredByDate(
  calendar: CalendarWithDetails | null,
  me: Participant | null,
  votesByDate: Set<string>,
  selectedDates: Set<string>,
): Map<string, Set<string>> {
  const map = new Map<string, Set<string>>();
  if (!calendar) return map;
  for (const a of calendar.availability) {
    if (me && a.participantId === me.id) continue;
    if (!a.preferred) continue;
    const set = map.get(a.date) ?? new Set<string>();
    set.add(a.participantId);
    map.set(a.date, set);
  }
  if (me) {
    for (const date of votesByDate) {
      if (!selectedDates.has(date)) continue;
      const set = map.get(date) ?? new Set<string>();
      set.add(me.id);
      map.set(date, set);
    }
  }
  return map;
}

// Drives the cell's note-dot colour rule: when exactly one participant has a
// note on a given date, the dot takes that participant's colour; otherwise
// it's neutral.
export function buildNoteAuthorsByDate(
  calendar: CalendarWithDetails | null,
  me: Participant | null,
  notesByDate: Map<string, string>,
): Map<string, Set<string>> {
  const map = new Map<string, Set<string>>();
  if (!calendar) return map;
  for (const a of calendar.availability) {
    if (me && a.participantId === me.id) continue;
    if (!a.note) continue;
    const set = map.get(a.date) ?? new Set<string>();
    set.add(a.participantId);
    map.set(a.date, set);
  }
  if (me) {
    for (const [date, text] of notesByDate) {
      if (!text) continue;
      const set = map.get(date) ?? new Set<string>();
      set.add(me.id);
      map.set(date, set);
    }
  }
  return map;
}

// The *text* of each note keyed by (date → participantId → text). Drives the
// Summary notes pill.
export function buildNotesByDateAndAuthor(
  calendar: CalendarWithDetails | null,
  me: Participant | null,
  notesByDate: Map<string, string>,
): Map<string, Map<string, string>> {
  const map = new Map<string, Map<string, string>>();
  if (!calendar) return map;
  for (const a of calendar.availability) {
    if (me && a.participantId === me.id) continue;
    if (!a.note) continue;
    const inner = map.get(a.date) ?? new Map<string, string>();
    inner.set(a.participantId, a.note);
    map.set(a.date, inner);
  }
  if (me) {
    for (const [date, text] of notesByDate) {
      if (!text) continue;
      const inner = map.get(date) ?? new Map<string, string>();
      inner.set(me.id, text);
      map.set(date, inner);
    }
  }
  return map;
}

// Header meta pills: "{n} people" and either "{n} weekends" (when every
// distinct upcoming date anyone marked is a Sat/Sun) or "{n} days".
export function calendarMetaLabels(
  calendar: CalendarWithDetails,
  selectedDates: Set<string>,
  today: string,
): { peopleLabel: string; dateLabel: string } {
  const participantCount = calendar.participants.length;
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
  const weekends = Math.ceil(dateCount / 2);
  const dateLabel =
    dateCount === 0
      ? "no dates yet"
      : allWeekends
        ? `${weekends} ${weekends === 1 ? "weekend" : "weekends"}`
        : `${dateCount} ${dateCount === 1 ? "day" : "days"}`;
  const peopleLabel = `${participantCount} ${participantCount === 1 ? "person" : "people"}`;
  return { peopleLabel, dateLabel };
}

export interface OwnAvailability {
  // The current participant, or null when not joined.
  participant: Participant | null;
  selectedDates: Set<string>;
  votesByDate: Set<string>;
  notesByDate: Map<string, string>;
}

export interface AvailabilityMaps {
  freeByDate: Map<string, Set<string>>;
  voteCountsByDate: Map<string, number>;
  preferredByDate: Map<string, Set<string>>;
  noteAuthorsByDate: Map<string, Set<string>>;
  notesByDateAndAuthor: Map<string, Map<string, string>>;
}

// Memoised wrapper: each map recomputes only when its own inputs change.
export function useAvailabilityMaps(
  calendar: CalendarWithDetails | null,
  { participant, selectedDates, votesByDate, notesByDate }: OwnAvailability,
): AvailabilityMaps {
  const freeByDate = useMemo(
    () => buildFreeByDate(calendar, participant, selectedDates),
    [calendar, participant, selectedDates],
  );
  const voteCountsByDate = useMemo(
    () => buildVoteCountsByDate(calendar, participant, votesByDate, selectedDates),
    [calendar, participant, votesByDate, selectedDates],
  );
  const preferredByDate = useMemo(
    () => buildPreferredByDate(calendar, participant, votesByDate, selectedDates),
    [calendar, participant, votesByDate, selectedDates],
  );
  const noteAuthorsByDate = useMemo(
    () => buildNoteAuthorsByDate(calendar, participant, notesByDate),
    [calendar, participant, notesByDate],
  );
  const notesByDateAndAuthor = useMemo(
    () => buildNotesByDateAndAuthor(calendar, participant, notesByDate),
    [calendar, participant, notesByDate],
  );
  return { freeByDate, voteCountsByDate, preferredByDate, noteAuthorsByDate, notesByDateAndAuthor };
}
