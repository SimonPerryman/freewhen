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
}

export interface CalendarWithDetails {
  calendar: Calendar;
  participants: Participant[];
  availability: Availability[];
  participantCount: number;
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
