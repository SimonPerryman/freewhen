export type Visibility = "open" | "submit_to_view";

export interface Calendar {
  id: string;
  name: string;
  emoji?: string;
  ownerId: string;
  visibility: Visibility;
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
}

export interface Availability {
  participantId: string;
  calendarId: string;
  date: string;
  note?: string;
}

export interface CalendarWithDetails {
  calendar: Calendar;
  participants: Participant[];
  availability: Availability[];
  participantCount: number;
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
  visibility?: Visibility;
}

export interface UpdateCalendarRequest {
  name?: string;
  visibility?: Visibility;
}

export interface AddParticipantRequest {
  name: string;
}

export interface SetAvailabilityRequest {
  participantId: string;
  dates: string[];
  // Partial override map keyed by ISO date. Omitted entirely to preserve all
  // existing notes ("don't touch" semantics). A key with an empty string clears
  // that date's note; a key with a non-empty value sets/replaces it.
  notes?: Record<string, string>;
}

export interface User {
  id: string;
  email: string;
  name: string;
  isAdmin: boolean;
}

export interface ApiError {
  error: string;
}

export type MyCalendarRole = "owner" | "joiner";

export interface MyCalendarListItem {
  id: string;
  name: string;
  emoji: string;
  visibility: Visibility;
  createdAt: string;
  updatedAt: string;
  finalisedDate: string | null;
  finalisedAt: string | null;
  participantCount: number;
  myRole: MyCalendarRole;
}

export interface MyCalendarsResponse {
  calendars: MyCalendarListItem[];
  hasMore: boolean;
}
