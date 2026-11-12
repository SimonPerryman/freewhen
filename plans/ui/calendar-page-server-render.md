# Calendar page server render

Status: done

## Goal

A Lighthouse audit of a shared calendar link (`/calendars/[id]`) found the
page paints "Loading…" until the JS bundle has loaded **and** a client fetch
to the Cloud Run API has completed (which can include a cold start). The
server already fetches the calendar for `generateMetadata`, so pass that
data into the client and let the first paint show the title and grid.

Builds on the hook split in `plans/tech-debt/calendar-page-client-split.md`.

## Plan

1. **One server fetch per request.** New `src/lib/serverCalendar.ts`
   exports `getCalendarForServer = cache(async (id) => …)` (React `cache()`),
   used by both `generateMetadata` and the page component, so a request
   fetches once.
   - Plain `fetch` with **no credentials and no forwarded headers**. It is
     an anonymous request, so it can only see what an anonymous visitor can.
     It never reads `cookies()` / `headers()` and never sends
     `X-Calendar-Access` or `X-Participant-Token`.
   - `cache: "no-store"`: availability changes live (other participants
     edit it, SSE pushes updates), and a stale render followed by a client
     correction would flash old data. The route is already dynamic (`ƒ`),
     so the only cost is the one API call we were already making for
     metadata. `cache()` dedupes per request; nothing is shared across
     requests or users.
   - Returns `null` on any non-2xx or network error. Metadata keeps its
     current fallbacks (the gated stub still yields "Untitled Calendar").
2. **Only open calendars get initial data.** The page passes `initialData`
   only when the response is a full calendar (`!requiresPassphrase`).
   Passphrase-gated (200 + `requires_passphrase`), 404, 410, 5xx, and
   network failures all pass nothing, and the client takes the existing
   path ("Loading…" → client fetch with the viewer's access token, cookies
   and participant token). The anonymous response only differs from a
   credentialed one in owner-only `passphrase`, which is absent whenever
   there's no passphrase, so nothing hidden can leak.
3. **Client hydrate mode** (`useCalendarData`): with `initialData`, the
   first load starts with `loading=false` and the server calendar, skips the
   data fetch, and runs the same post-load step (`onLoaded`: identity
   resolution, claim, seeding) against the server snapshot. Live updates
   then work exactly as before. The SSE stream opens once the calendar is
   in state, and its `open` event triggers the credentialed `refresh()`.
   `retryLoad` and the passphrase-unlock path are still full client loads.
4. **No identity flash**: the server can't know who "you" are (that's in
   localStorage). Every `!currentParticipant` block (join form, "full",
   "finalised" and "share the link" hints) waits until identity has
   resolved, so a returning participant never sees a join form flash.
   On the old path this resolves behind the loading screen, so nothing
   visible changes there.
5. **No hydration mismatches on dates**: "today" differs between the server
   (UTC) and the browser's timezone. A new `useToday()` uses
   `useSyncExternalStore` with the server's `today` (passed from
   `page.tsx`) as the server snapshot. Hydration uses the server value, then
   React re-renders with the browser's local date. `CalendarGrid`,
   `BestDay`, `SummaryView` and the header meta pills read `useToday()`
   instead of `new Date()` during render.
6. **Lazy-load the page's overlays** with `next/dynamic`: `DayDetailSheet`,
   `DuplicateCalendarSheet`, `CalendarSettingsSheet`, `ConfirmDialog` (via
   `CalendarOwnerDialogs`) and `PassphraseGate`. They only render after
   user interaction (or for gated calendars), so they don't need to be in
   the first-load chunk.
7. Tests: initial data renders without "Loading…" and without a client
   `getCalendar`; a passphrase-gated / failed server fetch falls back to the
   client fetch; metadata + page share one fetch (`cache()` dedupe); the
   server fetch sends no credentials or cookies; `useToday` hydrates with
   the server value.

## Log

- 2026-11-12 14:45 — Created plan
- 2026-11-12 14:46 — Implemented: `serverCalendar.ts` (cached, anonymous,
  `no-store`, 8s timeout so a hung API degrades to the client path),
  `useToday` + `ServerTodayProvider`, hydrate mode in `useCalendarData`,
  `identityResolved` gate in `useParticipantIdentity`, `loading.tsx` as the
  route Suspense fallback (the server fetch no longer blocks the shell),
  `key={id}` on the client so a different calendar remounts. The SSE stream
  now waits for identity, so a server-rendered page opens one stream already
  tagged with the participant instead of reconnecting.
- 2026-11-12 14:46 — Lazy overlays: PassphraseGate, DayDetailSheet,
  DuplicateCalendarSheet and CalendarOwnerDialogs (settings sheet + confirm
  dialogs) via `next/dynamic`. The owner overlays mount only while one is
  open; the day-sheet chunk is prefetched once you've joined. One existing
  test line changed: the settings-sheet test now `findByRole`s the Rename
  button because the sheet appears asynchronously.
- 2026-11-12 14:48 — Tests: 20 new tests (server fetch is anonymous / no-store
  / encoded; metadata + page share one fetch; gated and failed reads pass no
  initial data; first render has no "Loading…" and no client fetch; no join
  form flash; real `renderToString` → `hydrateRoot` with the server a day
  behind the browser hydrates with zero recoverable errors). A temporary
  negative control (hydrating with a different "today") does fail with
  "Hydration failed", so the test really does catch mismatches.
- 2026-11-12 14:50 — `pnpm build` route table unchanged (`ƒ /calendars/[id]`,
  dynamic before and after). Up-front JS for the route (root main files +
  route entry chunks): 626.2 kB raw / 177.8 kB gzip → 612.9 kB / 174.8 kB.
  Smoke test with `next start` against a mock API: open calendar HTML
  contains the title and participants; gated and 404 calendars contain
  neither (client fallback); one API call per page view (metadata + page
  deduped); the API saw no cookie, access token or participant token even
  though the browser request carried a session cookie.
