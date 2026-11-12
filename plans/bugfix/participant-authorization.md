# Participant authorization (M1 / L2)

Status: done

## Goal

Close an impersonation hole: anyone holding a calendar link could act as any
participant on it. `participant_id` came from the request body on
`PUT /availability`, `POST /availability/import` and
`PATCH /participants/{pid}`, and the service's "caller == target" check
compared two caller-supplied values. Participant ids are public in
`GET /calendars/{id}`, so `PUT /calendars/X/availability
{"participant_id":"<owner>","dates":[]}` wiped someone else's availability.
Import only checked that *someone* was logged in.

Also (L2): `POST /participants/{pid}/claim` skipped the passphrase gate and
returned the participant's stored email unscrubbed. Claim also had no
ownership check at all, so a signed-in attacker could claim an anonymous
participant and then edit it as "themselves".

## Design

A single service-layer check, `authorizeParticipantWrite`, decides every
participant-scoped mutation. Caller identity is the session user plus an
optional raw participant token (`X-Participant-Token` request header) — never
a body field.

1. **Linked participant** (`user_id` set): the session user must equal
   `user_id`.
2. **Anonymous participant with a token hash**: the request must carry the
   matching token. Compared as SHA-256 hex digests with
   `crypto/subtle.ConstantTimeCompare`.
3. **Anonymous participant with no token hash** (`token_hash IS NULL`): always
   403. Tokens are issued only at join, so there is no trust-on-first-use
   path. Production has no users yet, so pre-existing rows need no
   migration path.
4. **Calendar owner** keeps the powers it already had: rename / email edit of
   any participant (and remove, which is a separate owner-only endpoint).
   The owner can **not** write another participant's availability — the UI
   never did this, and it was only possible because of the bug.

Token issuance: `AddParticipant` for an anonymous joiner mints a 32-byte
`crypto/rand` token (base64url), stores only the hash in the new
`participants.token_hash` column (migration 027) in the same INSERT, and
returns the raw token once. Signed-in joiners get no token (rule 1 covers
them).

Transport: the backend returns the token in an `X-Participant-Token`
**response** header on the anonymous join response only. The frontend's `request()` helper captures it for any
calendar-scoped request and stores it in localStorage
(`freewhen:participantTokens`, map calendarId → token — kept separate from
the existing `freewhen:<calendarId>` participant record because that record
is overwritten wholesale in several places), and sends it back on every
calendar-scoped request. CORS allows the request header and exposes the
response header.

Claim: requires the passphrase gate, then rule 2 on the (unlinked) target;
the returned participant has `email` scrubbed.

### Trade-offs

- **Duplicate calendar copies only signed-in participants.** A copied
  anonymous row would have no token (uneditable forever) while holding the
  name (names are unique per calendar) and blocking the real person from
  rejoining. Anonymous participants rejoin the copy via its link; the
  Duplicate sheet's "Copy participants" hint says so.
- **Lost token = locked out** for anonymous participants (cleared storage,
  new device). Recovery: owner removes the row and they rejoin, or they sign
  in on the original device (claim). Same model as the existing
  passphrase-access token.
- The legacy `participant_id` body field on `PATCH /participants/{pid}` is
  now ignored.

## Plan

1. Migration `027_participant_token_hash.sql`: nullable `token_hash TEXT`.
2. Store: `AddParticipantWithTokenHash`, `GetParticipantForAuth`;
   `DuplicateCalendar` copies only user-linked participants.
3. Service: `ParticipantCaller`, token helpers, `authorizeParticipantWrite`;
   rewire AddParticipant / SetAvailability / ImportAvailability /
   RenameParticipant / UpdateParticipantEmail / ClaimParticipant.
4. Handlers: read `X-Participant-Token`, emit it on anonymous join; claim gets
   `requireCalendarAccess` + email scrub.
5. CORS headers.
6. Frontend: `participantToken.ts`, capture/send in `request()`, claim sends
   `calendarId`, drop `participant_id` from rename/email bodies.
7. Tests: service unit tests, handler tests, integration tests (403 on
   another participant's id for set/import/rename/email, tokenless anonymous
   row, claim, duplicate copies only linked participants),
   frontend tests.
8. Docs: `docs/api.md`, `docs/onboarding.html`.

## Log

- 2026-11-12 10:48 — Created plan; verified M1 and L2 against `origin/main`.
- 2026-11-12 10:55 — Implemented migration 027, store methods, `service.authorizeParticipantWrite` (+ rewired AddParticipant / SetAvailability / ImportAvailability / RenameParticipant / UpdateParticipantEmail / ClaimParticipant), handler header plumbing, claim passphrase gate + email scrub, CORS allow/expose headers. Also found: claim had no ownership check (signed-in attacker could claim an anon row, then edit it) — now requires the participant token.
- 2026-11-12 10:58 — Frontend: `participantToken.ts`, `request()` captures/sends `X-Participant-Token`, claim passes `calendarId`, rename/email drop `participant_id`. Vitest 266/266 green, oxlint clean.
- 2026-11-12 11:00 — Tests: service unit (auth matrix, TOFU, race loser, claim, join token), handler unit (403 mapping, header in/out, claim gate + scrub), store + server integration (403 for set/import/rename/email/claim with another participant's id, TOFU). Integration harness replays participant tokens like a browser. Updated `docs/api.md`, `docs/onboarding.html`. Go toolchain unavailable on this machine and CI not starting — backend lint/unit/integration **not run**; PR opened as draft.
- 2026-11-12 11:20 — Go toolchain became available: `go vet` (with and without `integration` tag) clean; `go test ./...` and `go test -p 1 -tags integration ./...` all pass, including the new auth tests. golangci-lint: only CRLF gofmt noise and pre-existing errcheck findings in integration-tag test helpers. PR marked ready.
- 2026-11-12 12:10 — Removed trust-on-first-use (owner confirmed no production users, so no legacy rows to protect): anonymous rows with NULL `token_hash` are now always 403; `authorizeParticipantWrite` returns `(Participant, error)`; SetAvailability / ImportAvailability / RenameParticipant / UpdateParticipantEmail no longer return a token (join is the only issuer); dropped `SetParticipantTokenHashIfUnset`. `DuplicateCalendar` now copies only user-linked participants; Duplicate sheet hint updated. TOFU tests replaced with tokenless-row-403 tests (service, integration); `docs/api.md`, `docs/onboarding.html` updated.
- 2026-11-12 12:25 — Rebased onto main (#70–#72). Conflicts only in `frontend/src/lib/api.ts` (kept `API_BASE_URL` import and participant-token imports) and `api.test.ts` (kept `verifyMagicLink` and participant-token test imports). Verified: `go vet` (both tags) clean, `go test ./...` and `go test -p 1 -tags integration ./...` pass, golangci-lint clean on touched files, vitest 319/319, oxlint 0/0.
