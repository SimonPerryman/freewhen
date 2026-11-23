# Default Name From Signed-In User

Status: done

## Goal

Prefill the join form's name input so users rarely have to type their name. Covers three cases:

1. **Signed-in user** — prefill with `user.name` from the session. Already implemented as a side effect of `plans/v2/t1/participant-user-linking.md` (see `CalendarPageClient.tsx:225-228`), but needs confirmation-via-tests and a retroactive note. Magic-link users without a name fall back to empty — that's fine, the name field stays editable.
2. **Returning anon user on a calendar they've already joined** — already works via per-calendar `localStorage` (`STORAGE_PREFIX + calendarId`).
3. **Anon user opening a fresh calendar link in a browser they've joined from before** — **not covered today.** Per-calendar storage doesn't carry across. We want a global "last used name" fallback so the 4th calendar someone receives doesn't make them retype "Sam" again.

The v2-priorities line strictly mentions the signed-in case, but the anon-carryover is the bigger UX win and is cheap, so we do both in one pass.

## Scope

**In scope:**
- Global `freewhen:lastName` key in `localStorage`, written on successful join, read as the final non-empty fallback for the join form default.
- Update `storedName` precedence in `CalendarPageClient` to: (1) stale per-calendar stored participant name, (2) signed-in `user.name`, (3) global last-used name, (4) empty.
- Write on every successful join (anon or signed-in — it's fine if signed-in users also update this; it's still correct).
- Tests: cover the new fallback path and confirm signed-in prefill still wins.
- Mark the feature as shipped in `docs/v2-priorities.md`.

**Out of scope:**
- Reactive prefill after a sign-in mid-session (current prefill only runs on initial load; not worth chasing — users who sign in typically do it before opening calendar links).
- Server-side "last used name" per user. The signed-in case already covers users who persist.
- Honouring the global fallback on the stale-participant branch (`CalendarPageClient.tsx:220-222`) — leave that as-is; if they have a stale participant on *this* calendar, that name is still the best guess.
- Clearing the global name on sign-out. It's not sensitive (first names are not PII we actively protect here — they go in cell overlays publicly), and clearing would surprise users who intentionally re-opened a browser.

## Design Decisions

**Storage key: `freewhen:lastName`** (flat string, no JSON). Keep it boring. Distinct from the per-calendar `STORAGE_PREFIX` entries so there's no risk of collision.

**Write on successful join, not on every keystroke.** Writing on keystroke means a half-typed name leaks across calendars. Write only after `addParticipant` resolves.

**Precedence rationale.** Signed-in name beats the global cache because it's the source of truth the user can actively edit (`UserMenu`). The global cache is only there for anon users who never sign in.

**Length cap.** The stored value is already capped by the join form's `maxLength` on input, and the server's length check on submit. The read path can be defensive (truncate to `MAX_PARTICIPANT_NAME_LENGTH`) in case the constant ever shrinks — one-liner.

**No migration.** Anyone without the key gets the current "empty input" experience until their first join. That's the worst case and it's exactly status quo.

**SSR-safety.** `readStorage` already guards `localStorage` via try/catch; mirror that for the global read. Don't touch `localStorage` outside a `useEffect` / event handler.

## Implementation

### Phase 1 — Global name storage helpers

In `CalendarPageClient.tsx` (or a new `src/lib/lastName.ts` if it's cleaner — prefer inline unless a second caller appears):

```ts
const LAST_NAME_KEY = "freewhen:lastName";

function readLastName(): string {
  try {
    const raw = localStorage.getItem(LAST_NAME_KEY);
    if (!raw) return "";
    return raw.slice(0, MAX_PARTICIPANT_NAME_LENGTH);
  } catch {
    return "";
  }
}

function writeLastName(name: string): void {
  try {
    localStorage.setItem(LAST_NAME_KEY, name);
  } catch {
    // ignore quota / disabled storage
  }
}
```

### Phase 2 — Wire fallback into `storedName` resolution

In the load effect around `CalendarPageClient.tsx:225-228`, after the signed-in prefill block, add:

```ts
// 5. Fall back to last-used name from any prior join in this browser.
if (!resolved && !storedNameSet) {
  const last = readLastName();
  if (last) setStoredName(last);
}
```

Refactor slightly so that the "did we already set storedName?" check is explicit rather than relying on ordering. Simplest shape: compute the candidate once at the end of the branch (`stored?.name ?? user?.name ?? readLastName() ?? ""`) and call `setStoredName` once.

### Phase 3 — Write on successful join

In `handleJoin` (`CalendarPageClient.tsx:845`), after `writeStorage(calendarId, ...)`:

```ts
writeLastName(name);
```

### Phase 4 — Tests

In `CalendarPageClient.test.tsx`:
- New test: no stored participant, no signed-in user, `localStorage.getItem("freewhen:lastName")` returns `"Sam"` → `JoinForm` receives `defaultName="Sam"`.
- New test: signed-in user with `name="Alice"` AND `lastName="Sam"` in storage → `defaultName="Alice"` (signed-in wins).
- New test: join flow writes `"freewhen:lastName"` on success.
- Existing tests stay green — the stale-participant branch is untouched.

Reuse the existing `localStorageMock` — add handling for the new key alongside the per-calendar ones (the mock is a generic `Map`-shape, so likely no changes needed to the mock itself).

### Phase 5 — Docs

- `docs/v2-priorities.md`: strike through "Default name from signed-in user", note shipped, reference this plan.
- Log in this plan file.

## Testing

Manual QA on mobile:
1. Fresh browser, anon — join calendar A as "Sam". Open calendar B link → input prefills "Sam".
2. Edit to "Sammy" and join B → open calendar C → prefill is "Sammy".
3. Sign in as a Google user named "Alice" → open calendar D → prefill is "Alice" (signed-in wins).
4. Sign in via magic-link (no name) → open calendar E → prefill falls back to "Sammy" (global cache).
5. Clear site data → open any calendar → input empty.

## Build Order

1. Phase 1 helpers + Phase 2 fallback read + Phase 3 write-on-join — single commit, tightly coupled.
2. Phase 4 tests.
3. Phase 5 docs update + mark shipped.

## Log

- 2026-04-21 — Created plan. Signed-in prefill already shipped as side effect of participant-user-linking (commit `e6a64d3`); this plan's real delivery is the anon cross-calendar fallback.
- 2026-04-21 — Implemented all phases. Added `freewhen:lastName` helpers + write-on-join in `CalendarPageClient.tsx`; refactored storedName resolution into a single `setStoredName` call with precedence stale → signed-in → global last-name → empty. Added three tests in `CalendarPageClient.test.tsx` covering the new fallback, signed-in precedence, and write-on-join. Marked shipped in `docs/v2-priorities.md`.
