// Per-calendar access tokens issued by POST /calendars/{id}/unlock. Stored in
// localStorage under a single key so a tab sees every calendar the user has
// unlocked, and so tokens survive page reloads and navigation. Cleared on
// any 401 response from a gated endpoint.
//
// See plans/password-protected-calendars.md for why this lives in
// localStorage + a custom header rather than per-calendar cookies.

const STORAGE_KEY = "freewhen:calendarAccess";

type AccessMap = Record<string, string>;

function read(): AccessMap {
  if (typeof localStorage === "undefined") return {};
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    if (parsed && typeof parsed === "object") return parsed as AccessMap;
    return {};
  } catch {
    return {};
  }
}

function write(map: AccessMap): void {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(map));
  } catch {
    // Quota exceeded or storage disabled — nothing to do; the user will be
    // re-gated on the next request.
  }
}

export function getCalendarAccessToken(calendarId: string): string | null {
  return read()[calendarId] ?? null;
}

export function setCalendarAccessToken(calendarId: string, token: string): void {
  const map = read();
  map[calendarId] = token;
  write(map);
}

export function clearCalendarAccessToken(calendarId: string): void {
  const map = read();
  if (!(calendarId in map)) return;
  delete map[calendarId];
  write(map);
}
