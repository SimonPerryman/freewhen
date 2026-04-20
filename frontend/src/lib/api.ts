import type {
  AddParticipantRequest,
  Calendar,
  CalendarWithDetails,
  CreateCalendarRequest,
  CreateCalendarResponse,
  Participant,
  SetAvailabilityRequest,
  UpdateCalendarRequest,
  User,
} from "./types";

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  public requestId: string | null;

  constructor(
    public status: number,
    message: string,
    requestId: string | null = null,
  ) {
    super(message);
    this.name = "ApiError";
    this.requestId = requestId;
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

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    ...options,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });

  if (!response.ok) {
    const body = (await response.json()) as { error: string };
    throw new ApiError(response.status, body.error, response.headers.get("X-Request-ID"));
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
  return request<CalendarWithDetails>(`/calendars/${id}${query}`);
}

export async function addParticipant(
  calendarId: string,
  req: AddParticipantRequest,
): Promise<Participant> {
  return request<Participant>(`/calendars/${calendarId}/participants`, {
    method: "POST",
    body: JSON.stringify(toSnakeCaseKeys(req)),
  });
}

export async function setAvailability(
  calendarId: string,
  req: SetAvailabilityRequest,
): Promise<void> {
  await request(`/calendars/${calendarId}/availability`, {
    method: "PUT",
    body: JSON.stringify(toSnakeCaseKeys(req)),
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

export async function removeParticipant(
  calendarId: string,
  participantId: string,
): Promise<void> {
  await request(`/calendars/${calendarId}/participants/${participantId}`, {
    method: "DELETE",
  });
}

export async function getMe(): Promise<User> {
  return request<User>("/auth/me");
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
