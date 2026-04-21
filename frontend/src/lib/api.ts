import type {
  AddParticipantRequest,
  Calendar,
  CalendarWithDetails,
  CreateCalendarRequest,
  CreateCalendarResponse,
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

export async function getCalendar(id: string, as?: string): Promise<CalendarWithDetails> {
  const query = as ? `?as=${encodeURIComponent(as)}` : "";
  return request<CalendarWithDetails>(`/calendars/${id}${query}`, { calendarId: id });
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

export function getMagicLinkCallbackUrl(token: string): string {
  return `${BASE_URL}/auth/magic-link/callback?token=${encodeURIComponent(token)}`;
}
