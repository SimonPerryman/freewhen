export interface Calendar {
  id: string;
  name: string;
  emoji?: string;
  ownerId: string;
  createdAt: string;
  updatedAt: string;
  finalisedDate?: string;
  finalisedAt?: string;
  hasPassphrase?: boolean;
  // Present only when the authenticated owner fetches their own calendar.
  passphrase?: string;
}

export interface Participant {
  id: string;
  calendarId: string;
  name: string;
  colour: string;
  userId?: string;
  email?: string | null;
}

export interface Availability {
  participantId: string;
  calendarId: string;
  date: string;
  note?: string;
  preferred?: boolean;
}

export interface CalendarWithDetails {
  calendar: Calendar;
  participants: Participant[];
  availability: Availability[];
  // Present (and true) when the calendar is gated and the caller has not yet
  // provided a valid access token. When set, `participants` / `availability`
  // are empty and `calendar` is a stub containing only id + hasPassphrase.
  requiresPassphrase?: boolean;
}

export interface CreateCalendarResponse extends Calendar {
  participant?: Participant;
}

export interface CreateCalendarRequest {
  name: string;
}

export interface DuplicateCalendarRequest {
  name?: string;
  includeParticipants?: boolean;
}

export interface UpdateCalendarRequest {
  name?: string;
  // One of PICKABLE_EMOJIS (lib/emoji.ts); the server rejects anything else.
  emoji?: string;
}

export interface AddParticipantRequest {
  name: string;
  email?: string | null;
}

export interface SetAvailabilityRequest {
  participantId: string;
  dates: string[];
  // Partial override map keyed by ISO date. Omitted entirely to preserve all
  // existing notes ("don't touch" semantics). A key with an empty string clears
  // that date's note; a key with a non-empty value sets/replaces it.
  notes?: Record<string, string>;
  // Full list of dates the participant prefers. Omitted to preserve existing
  // votes; present (even empty) overwrites all votes for dates in `dates`.
  preferred?: string[];
}

export interface User {
  id: string;
  email: string;
  name: string;
  isAdmin: boolean;
  // Timestamp the user acknowledged the Google Calendar import privacy
  // interstitial. Null/undefined → show the interstitial before the first
  // import; set → skip straight to OAuth.
  calendarImportConsentAckAt?: string;
}

export interface ImportSuggestions {
  // false when there is no pending import (the normal state). Absent on
  // responses from backends that predate the field.
  pending?: boolean;
  free: string[];
  busy: string[];
}

export interface ApiError {
  error: string;
}

export type MyCalendarRole = "owner" | "joiner";

export interface MyCalendarListItem {
  id: string;
  name: string;
  emoji: string;
  createdAt: string;
  updatedAt: string;
  finalisedDate: string | null;
  finalisedAt: string | null;
  participantCount: number;
  myRole: MyCalendarRole;
}

export interface Invite {
  id: string;
  calendarId: string;
  name: string;
  matchedParticipantId: string | null;
  createdAt: string;
}

export interface NotJoinedEntry {
  inviteId: string;
  name: string;
}

export interface JoinedNotRespondedEntry {
  participantId: string;
  name: string;
}

export interface MissingView {
  invites: Invite[];
  notJoined: NotJoinedEntry[];
  joinedNotResponded: JoinedNotRespondedEntry[];
}

export interface SkippedName {
  name: string;
  reason: string;
}

export interface InvalidName {
  name: string;
  reason: string;
}

export interface AddInvitesResult {
  added: Invite[];
  skipped: SkippedName[];
  invalid: InvalidName[];
}

export interface MyCalendarsResponse {
  calendars: MyCalendarListItem[];
  hasMore: boolean;
}
