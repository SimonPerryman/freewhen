import type {
  AddInvitesResult,
  AddParticipantRequest,
  Calendar,
  CalendarWithDetails,
  CreateCalendarRequest,
  CreateCalendarResponse,
  DuplicateCalendarRequest,
  ImportSuggestions,
  Invite,
  MissingView,
  MyCalendarsResponse,
  Participant,
  SetAvailabilityRequest,
  UpdateCalendarRequest,
  User,
} from "./types";
import {
  clearCalendarAccessToken,
  getCalendarAccessToken,
  setCalendarAccessToken,
} from "./calendarAccess";

const ACCESS_HEADER = "X-Calendar-Access";

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  public requestId: string | null;
  public code: string | null;

  constructor(
    public status: number,
    message: string,
    requestId: string | null = null,
    code: string | null = null,
  ) {
    super(message);
    this.name = "ApiError";
    this.requestId = requestId;
    this.code = code;
  }
}

function toSnakeCase(str: string): string {
  return str.replace(/[A-Z]/g, (letter) => `_${letter.toLowerCase()}`);
}

function toCamelCase(str: string): string {
  return str.replace(/_([a-z])/g, (_, letter: string) => letter.toUpperCase());
}

function convertKeys<T>(obj: unknown, convertKey: (key: string) => string): T {
  if (Array.isArray(obj)) {
    return obj.map((item) => convertKeys(item, convertKey)) as T;
  }
  if (obj !== null && typeof obj === "object") {
    const result: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(obj)) {
      result[convertKey(key)] = convertKeys(value, convertKey);
    }
    return result as T;
  }
  return obj as T;
}

function toSnakeCaseKeys(obj: unknown): unknown {
  return convertKeys(obj, toSnakeCase);
}

function toCamelCaseKeys<T>(obj: unknown): T {
  return convertKeys<T>(obj, toCamelCase);
}

async function request<T>(
  path: string,
  options: RequestInit & { calendarId?: string } = {},
): Promise<T> {
  const { calendarId, ...rest } = options;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(rest.headers as Record<string, string> | undefined),
  };
  if (calendarId) {
    const token = getCalendarAccessToken(calendarId);
    if (token) headers[ACCESS_HEADER] = token;
  }

  const response = await fetch(`${BASE_URL}${path}`, {
    ...rest,
    credentials: "include",
    headers,
  });

  if (!response.ok) {
    // A 401 on a calendar-scoped request means the stored token is no longer
    // valid (revoked via passphrase rotation, expired, or never existed).
    // Drop it so the caller re-renders the gate.
    if (response.status === 401 && calendarId) {
      clearCalendarAccessToken(calendarId);
    }
    let message = `Request failed (${response.status})`;
    let code: string | null = null;
    try {
      const body = (await response.json()) as { error?: string; code?: string };
      if (body.error) message = body.error;
      code = body.code ?? null;
    } catch {
      // Non-JSON error body (e.g. Go's default 405 "Method Not Allowed").
      // Keep the status-based fallback message.
    }
    throw new ApiError(response.status, message, response.headers.get("X-Request-ID"), code);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const body: unknown = await response.json();
  return toCamelCaseKeys<T>(body);
}

export async function createCalendar(req: CreateCalendarRequest): Promise<CreateCalendarResponse> {
  return request<CreateCalendarResponse>("/calendars", {
    method: "POST",
    body: JSON.stringify(toSnakeCaseKeys(req)),
  });
}

export async function duplicateCalendar(
  id: string,
  req: DuplicateCalendarRequest = {},
): Promise<CreateCalendarResponse> {
  return request<CreateCalendarResponse>(`/calendars/${id}/duplicate`, {
    method: "POST",
    body: JSON.stringify(toSnakeCaseKeys(req)),
  });
}

export async function getCalendar(id: string): Promise<CalendarWithDetails> {
  return request<CalendarWithDetails>(`/calendars/${id}`, { calendarId: id });
}

// unlockCalendar submits the passphrase (shared or `?unlock=` magic-link
// value) and, on success, stores the returned access token so every
// subsequent calendar-scoped request carries it.
export async function unlockCalendar(calendarId: string, passphrase: string): Promise<void> {
  const { token } = await request<{ token: string }>(`/calendars/${calendarId}/unlock`, {
    method: "POST",
    body: JSON.stringify({ passphrase }),
  });
  setCalendarAccessToken(calendarId, token);
}

// rotateCalendarPassphrase generates (or regenerates) the calendar's
// passphrase. Owner only. Returns the new plaintext so the caller can
// display it and offer "copy link with passphrase".
export async function rotateCalendarPassphrase(calendarId: string): Promise<string> {
  const { passphrase } = await request<{ passphrase: string }>(
    `/calendars/${calendarId}/passphrase`,
    { method: "POST" },
  );
  return passphrase;
}

// clearCalendarPassphrase disables the gate. Owner only.
export async function clearCalendarPassphrase(calendarId: string): Promise<void> {
  await request(`/calendars/${calendarId}/passphrase`, { method: "DELETE" });
}

export async function addParticipant(
  calendarId: string,
  req: AddParticipantRequest,
): Promise<Participant> {
  return request<Participant>(`/calendars/${calendarId}/participants`, {
    method: "POST",
    body: JSON.stringify(toSnakeCaseKeys(req)),
    calendarId,
  });
}

export async function setAvailability(
  calendarId: string,
  req: SetAvailabilityRequest,
): Promise<void> {
  await request(`/calendars/${calendarId}/availability`, {
    method: "PUT",
    body: JSON.stringify(toSnakeCaseKeys(req)),
    calendarId,
  });
}

export async function finaliseCalendar(calendarId: string, date: string): Promise<Calendar> {
  return request<Calendar>(`/calendars/${calendarId}/finalise`, {
    method: "POST",
    body: JSON.stringify({ date }),
  });
}

export async function unfinaliseCalendar(calendarId: string): Promise<Calendar> {
  return request<Calendar>(`/calendars/${calendarId}/finalise`, {
    method: "DELETE",
  });
}

export async function updateCalendar(
  calendarId: string,
  patch: UpdateCalendarRequest,
): Promise<Calendar> {
  return request<Calendar>(`/calendars/${calendarId}`, {
    method: "PATCH",
    body: JSON.stringify(toSnakeCaseKeys(patch)),
  });
}

export async function renameCalendar(calendarId: string, name: string): Promise<Calendar> {
  return updateCalendar(calendarId, { name });
}

export async function renameParticipant(
  calendarId: string,
  participantId: string,
  name: string,
  callerParticipantId: string | null,
): Promise<Participant> {
  return request<Participant>(`/calendars/${calendarId}/participants/${participantId}`, {
    method: "PATCH",
    body: JSON.stringify({ name, participant_id: callerParticipantId ?? "" }),
    calendarId,
  });
}

// updateParticipantEmail sets or clears the participant's contact email. Pass
// null/empty to clear (in-app unsubscribe). Anonymous-only — signed-in users
// edit their email via account settings.
export async function updateParticipantEmail(
  calendarId: string,
  participantId: string,
  email: string | null,
  callerParticipantId: string | null,
): Promise<Participant> {
  return request<Participant>(`/calendars/${calendarId}/participants/${participantId}`, {
    method: "PATCH",
    body: JSON.stringify({ email: email ?? "", participant_id: callerParticipantId ?? "" }),
    calendarId,
  });
}

export interface NudgeResult {
  emailed: number;
  skipped: number;
}

export async function nudgeCalendar(calendarId: string): Promise<NudgeResult> {
  return request<NudgeResult>(`/calendars/${calendarId}/nudge`, {
    method: "POST",
  });
}

export async function removeParticipant(calendarId: string, participantId: string): Promise<void> {
  await request(`/calendars/${calendarId}/participants/${participantId}`, {
    method: "DELETE",
  });
}

export async function getMe(): Promise<User> {
  return request<User>("/auth/me");
}

export async function updateMe(patch: { name?: string }): Promise<User> {
  return request<User>("/auth/me", {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

export async function listMyCalendars(): Promise<MyCalendarsResponse> {
  return request<MyCalendarsResponse>("/users/me/calendars");
}

export async function logout(): Promise<void> {
  await request("/auth/logout", { method: "POST" });
}

// logoutEverywhere revokes every session for the user, then re-issues a
// fresh session for the current device. The session cookie is rotated
// transparently — the user stays logged in here. Audit F12.
export async function logoutEverywhere(): Promise<void> {
  await request("/auth/logout-all", { method: "POST" });
}

// ackCalendarImportConsent records that the user has seen the privacy
// interstitial shown before the first Google Calendar import. Idempotent —
// safe to call on every Continue click.
export async function ackCalendarImportConsent(): Promise<void> {
  await request("/auth/google/calendar/consent-ack", { method: "POST" });
}

// fetchImportSuggestions reads the single-use import-suggestions cookie the
// backend writes after a successful OAuth round-trip. Returns null if the
// cookie is missing (expired, never present, or already consumed).
export async function fetchImportSuggestions(
  calendarId: string,
): Promise<ImportSuggestions | null> {
  try {
    return await request<ImportSuggestions>(`/calendars/${calendarId}/import/suggestions`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return null;
    throw err;
  }
}

// importAvailabilityDays calls the "Accept all suggestions" shortcut —
// non-destructive merge of the given dates into the caller's availability.
export async function importAvailabilityDays(
  calendarId: string,
  participantId: string,
  days: string[],
): Promise<void> {
  await request(`/calendars/${calendarId}/availability/import`, {
    method: "POST",
    body: JSON.stringify({ participant_id: participantId, days }),
    calendarId,
  });
}

// googleCalendarConnectUrl builds the backend redirect that starts the
// incremental OAuth round-trip for Google Calendar free/busy access.
export function googleCalendarConnectUrl(
  calendarId: string,
  tz: string,
  returnUrl: string,
): string {
  const qs = new URLSearchParams({ calendar_id: calendarId, tz, return_url: returnUrl });
  return `${BASE_URL}/auth/google/calendar/connect?${qs.toString()}`;
}

export async function claimParticipant(
  calendarId: string,
  participantId: string,
): Promise<Participant> {
  return request<Participant>(`/calendars/${calendarId}/participants/${participantId}/claim`, {
    method: "POST",
  });
}

export async function sendMagicLink(email: string, returnUrl: string = "/"): Promise<void> {
  await request("/auth/magic-link", {
    method: "POST",
    body: JSON.stringify({ email, return_url: returnUrl }),
  });
}

export function getGoogleLoginUrl(returnUrl: string = "/"): string {
  return `${BASE_URL}/auth/google?return_url=${encodeURIComponent(returnUrl)}`;
}

// --- "Who's missing?" invite list (owner-only) ---

export async function getMissing(calendarId: string): Promise<MissingView> {
  return request<MissingView>(`/calendars/${calendarId}/missing`);
}

export async function addInvites(calendarId: string, names: string[]): Promise<AddInvitesResult> {
  return request<AddInvitesResult>(`/calendars/${calendarId}/invites`, {
    method: "POST",
    body: JSON.stringify({ names }),
  });
}

export async function renameInvite(
  calendarId: string,
  inviteId: string,
  name: string,
): Promise<Invite> {
  return request<Invite>(`/calendars/${calendarId}/invites/${inviteId}`, {
    method: "PUT",
    body: JSON.stringify({ name }),
  });
}

export async function deleteInvite(calendarId: string, inviteId: string): Promise<void> {
  await request(`/calendars/${calendarId}/invites/${inviteId}`, {
    method: "DELETE",
  });
}

export async function matchInvite(
  calendarId: string,
  inviteId: string,
  participantId: string | null,
): Promise<Invite> {
  return request<Invite>(`/calendars/${calendarId}/invites/${inviteId}/match`, {
    method: "POST",
    body: JSON.stringify({ participant_id: participantId }),
  });
}

// verifyMagicLink exchanges the magic-link token for a session. The frontend
// interstitial page calls this on user confirmation (button click), not on
// page load — that's what stops link prefetchers (Outlook SafeLinks, Slack
// unfurl, AV scanners) from burning the one-time token before the user
// clicks. Returns the validated server-side return URL to navigate to.
export async function verifyMagicLink(token: string): Promise<{ returnUrl: string }> {
  return request<{ returnUrl: string }>("/auth/magic-link/verify", {
    method: "POST",
    body: JSON.stringify({ token }),
  });
}
