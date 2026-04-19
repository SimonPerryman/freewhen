export interface Calendar {
  id: string;
  name: string;
  emoji?: string;
  createdAt: string;
}

export interface Participant {
  id: string;
  calendarId: string;
  name: string;
  colour: string;
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
}

export interface CreateCalendarRequest {
  name: string;
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
}

export interface ApiError {
  error: string;
}
