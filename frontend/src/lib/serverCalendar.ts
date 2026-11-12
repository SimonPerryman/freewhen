import { cache } from "react";
import { toCamelCaseKeys } from "./api";
import { API_BASE_URL } from "./config";
import { localToday } from "./localDate";
import type { CalendarWithDetails } from "./types";

// Upper bound on how long the server holds the stream waiting for the API
// (the route's loading.tsx is already on screen meanwhile). On timeout the
// client takes over with its own fetch, error panel and retry.
export const SERVER_FETCH_TIMEOUT_MS = 8000;

// Server-side read of a calendar for /calendars/[id] (metadata + first
// paint). See plans/ui/calendar-page-server-render.md.
//
// Deliberately an *anonymous* request: no cookies, no calendar access
// token, no participant token, no forwarded headers. It can only ever see
// what a logged-out visitor without the passphrase could see, so whatever it
// returns is safe to embed in the HTML.
//
// `cache: "no-store"` because availability changes live — a cached render
// would paint stale data and then visibly correct itself. React `cache()`
// dedupes the call within one server request (generateMetadata + the page),
// never across requests.
export const getCalendarForServer = cache(
  async (id: string): Promise<CalendarWithDetails | null> => {
    try {
      const res = await fetch(`${API_BASE_URL}/calendars/${encodeURIComponent(id)}`, {
        cache: "no-store",
        credentials: "omit",
        headers: { Accept: "application/json" },
        signal: AbortSignal.timeout(SERVER_FETCH_TIMEOUT_MS),
      });
      if (!res.ok) return null;
      return toCamelCaseKeys<CalendarWithDetails>(await res.json());
    } catch {
      return null;
    }
  },
);

// The calendar to hand the client as initial data, or null when the client
// must fetch for itself (passphrase-gated, missing, or the API was down).
export function initialCalendarData(data: CalendarWithDetails | null): CalendarWithDetails | null {
  if (!data || data.requiresPassphrase) return null;
  return data;
}

// The server's idea of "today", used as the hydration snapshot for
// useToday() so server and client markup agree. The client switches to its
// own local date immediately after hydrating.
export function serverToday(): string {
  return localToday();
}
