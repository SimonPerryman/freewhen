// Per-participant secrets for anonymous participants. The backend returns a
// token in the `X-Participant-Token` response header when an anonymous
// participant joins (or, for participants created before tokens existed, on
// their first successful write). We keep it in localStorage, keyed by
// calendar, and send it back on every calendar-scoped request so the backend
// can tell that this browser really is that participant — the participant id
// alone is public.
//
// Stored under its own key rather than inside the `freewhen:<calendarId>`
// participant record because that record is rewritten wholesale in several
// places. One browser holds at most one participant per calendar, so a
// calendar-keyed map is enough. See plans/bugfix/participant-authorization.md.

export const PARTICIPANT_TOKEN_HEADER = "X-Participant-Token";

const STORAGE_KEY = "freewhen:participantTokens";

type TokenMap = Record<string, string>;

function read(): TokenMap {
  if (typeof localStorage === "undefined") return {};
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    if (parsed && typeof parsed === "object") return parsed as TokenMap;
    return {};
  } catch {
    return {};
  }
}

function write(map: TokenMap): void {
  if (typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(map));
  } catch {
    // Quota exceeded or storage disabled — writes for this participant will
    // be rejected until storage works again.
  }
}

export function getParticipantToken(calendarId: string): string | null {
  return read()[calendarId] ?? null;
}

export function setParticipantToken(calendarId: string, token: string): void {
  const map = read();
  map[calendarId] = token;
  write(map);
}
