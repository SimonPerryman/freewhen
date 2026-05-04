# End-to-End Tests

Status: done

## Goal

Catch broken golden-path regressions before they reach users. Today we have unit and integration tests for backend code and component tests for the frontend, but nothing that exercises the full stack the way a real user does. A single broken handler, misnamed env var, or busted client-side route can ship green and only get caught when someone tries to use the product.

E2E tests gate PRs on the journeys that matter: a creator can sign in and create a calendar, joiners (anonymous or signed in) can submit availability, and the owner can finalise. If those work, the product works.

## Approach

- **Playwright** against a locally booted backend + frontend in CI. No staging env required.
- **Medium-grained journey tests**, not one mega-test or twenty tiny ones. Each spec covers one coherent user journey end-to-end.
- **Fixtures seed state via the API**, not the UI — only the behaviour under test goes through the browser. Keeps specs fast and focused.
- **Database**: start with the same embedded Postgres harness `make test-integration` already uses (zero new infra). Revisit Neon branches if we hit limits (parallelism, migrations-on-boot timing, etc).
- **File extension**: `.test.ts`, not `.spec.ts`, to match the rest of the repo. Set `testMatch: '**/*.test.ts'` explicitly in `playwright.config.ts`.

## Scope

In:

- Playwright skeleton in `frontend/e2e/`
- `make test-e2e` target that boots both stacks and runs the suite locally
- CI job on PRs (required check eventually)
- Functional specs covering the golden path (see table below)

Out:

- Visual regression / mobile-viewport screenshot testing — mobile-first is enforced at the CSS/component level; a functional spec at 375px hits the same code paths as one at 1280px. If we want a mobile guard later, the right tool is a separate visual-regression suite.
- Pure rendering tests of backend-computed state (e.g. "overlap highlights the right dates") — that's a unit test on the calculation + a component test on the rendering, not E2E.
- Performance / load testing
- Comprehensive page coverage (every error state, every form field)
- Prod smoke tests (separate decision; flagged as open question)

## Golden path

Driven by the actual API surface in `docs/api.md`:

**Creator (logged in):**

1. Sign in (Google OAuth or magic link)
2. `POST /calendars` — create a named calendar; auto-joined as participant
3. (Optional) `POST /calendars/{id}/invites` — add expected names
4. Share `/calendars/{id}` URL
5. `PUT /calendars/{id}/availability` — fill in own dates
6. `POST /calendars/{id}/finalise` — lock to a chosen date

**Joiner (anonymous OR logged in):**

1. Open share URL
2. `POST /calendars/{id}/participants` with name (auto-links user_id if signed in)
3. `PUT /calendars/{id}/availability` — pick dates, optional notes/preferred votes

**Anon → claim:**

- Joiner submits anonymously, later signs in, hits `POST /participants/{id}/claim` to attach the participant record to their account

## Specs

| Spec | Auth state | What it covers |
|------|-----------|----------------|
| `create-calendar.test.ts` | Logged in | Create a calendar through the UI, land on calendar page, see self as participant |
| `join-anonymous.test.ts` | Logged out | Open share link, join with name, no user_id linkage |
| `join-signed-in.test.ts` | Logged in (different user from creator) | Same flow, asserts user_id auto-link |
| `submit-availability.test.ts` | Logged out joiner | Pick dates, add a note, mark a preferred date, persists |
| `finalise.test.ts` | Logged in owner | Owner-only action; calendar locks; non-owner can't finalise |
| `claim-participant.test.ts` | Anon → sign in mid-flow | The one flow that genuinely needs both auth states |
| `magic-link-auth.test.ts` | Logged out → logged in via real magic link | Guards the magic-link UI flow end-to-end |

## Auth in tests

Real OAuth can't be meaningfully tested end-to-end (we'd be testing Google's UI). Magic-link is testable but slow. Approach:

- **Default for most specs**: a test-only backend endpoint, gated by `E2E_TEST_MODE=true` env (never set in prod), that creates a session for a given email. Specs call it via `request.post('/test/login', {email})` in a fixture. Fast, reliable.
- **One exception**: `magic-link-auth.test.ts` exercises the real magic-link flow against a local mailcatcher (e.g. MailHog) — request the link, poll the catcher's HTTP API for the email, extract the link, click it. This is the dedicated guard for the auth UI; every other spec uses the fast path.

## Plan

### Phase 1 — Skeleton + first spec (local only)

1. Install Playwright in `frontend/`: `pnpm add -D @playwright/test`, `pnpm exec playwright install chromium`.
2. Create `frontend/playwright.config.ts` with `webServer` blocks that boot the Go backend (against embedded Postgres, with `E2E_TEST_MODE=true`) and the Next.js frontend.
3. Add the `E2E_TEST_MODE`-gated test-login endpoint in the backend (`internal/handler/`) with a startup-time refusal if the flag is on outside dev/test.
4. Create `frontend/e2e/fixtures.ts` with:
   - `calendar` fixture (POSTs to API, tears down on test end)
   - `signedInPage` fixture (uses test-login endpoint)
5. Write `frontend/e2e/create-calendar.test.ts` as the first spec.
6. Add `make test-e2e` to the root `Makefile`.
7. Verify it runs green locally.

### Phase 2 — CI integration

1. New job `e2e` in `.github/workflows/` that runs on PRs.
2. Steps: checkout → setup Node/Go/pnpm → cache embedded Postgres binary → `pnpm exec playwright install --with-deps chromium` → `make test-e2e`.
3. Upload `playwright-report/` as artifact on failure.
4. Soft-launch (visible but non-blocking) for ~a week to flush out flakes, then mark required.

### Phase 3 — Expand coverage

Add the remaining specs from the table above, in roughly this order:

1. `join-anonymous.test.ts`
2. `submit-availability.test.ts`
3. `finalise.test.ts`
4. `join-signed-in.test.ts`
5. `claim-participant.test.ts`
6. `magic-link-auth.test.ts` (last — pulls in MailHog/catcher dependency)

### Phase 4 — Gate deploys

Once stable, add `e2e` to the required-checks list alongside `lint`, `test`, `test-integration`. Update `plans/infra/gate-deploy-on-ci.md` and `gate-vercel-deploy-on-ci.md` if needed.

## Test design notes

- **Fixtures hit the API directly** for setup. Only the behaviour under test goes through the UI.
- **No `waitForTimeout`**. Use `await expect(locator).toBeVisible()` etc — they auto-retry. Flakiness usually traces back to sleeping instead of waiting on state.
- **One coherent journey per spec**, multiple assertions along the way. When step 5 fails you want to know steps 1–4 worked.
- **Parallel-safe**: each spec creates its own calendar with a unique name, no shared global state.

### Selectors

Hierarchy, in order of preference:

1. **Role + accessible name** — `getByRole('button', { name: 'Create calendar' })`, `getByRole('dialog', { name: 'Calendar settings' })`. First choice for buttons, links, dialogs, live regions, headings.
2. **Label / placeholder / text** — `getByLabel('Email')` for inputs, `getByText(...)` for static content.
3. **`data-testid`** — fallback only, with naming convention `data-testid="kebab-case-purpose"`.

**If a spec needs a `data-testid` outside the cases below, treat it as a signal that something's missing an accessible name and fix the component instead** — don't paper over it with a testid. The E2E suite then doubles as a regression guard for accessibility (if someone removes an `aria-label`, the role+name selector breaks).

Genuinely testid-worthy elements in FreeWhen (after the a11y pass lands):

- **Calendar day cells** keyed by date — `data-testid="day-cell-2026-06-15"`. The visible content doesn't uniquely identify the cell.
- **Participant rows / colour swatches** keyed by participant id — visible name is user-controlled and unstable.

Don't strip testids in production builds — bytes are negligible, and removing them blocks running specs against a real prod build later (e.g. for prod smoke tests).

### Accessibility pass: resolved

`plans/ui/accessibility-pass.md` is marked **done**. Form labels + `aria-describedby`, icon-button `aria-label`s, and `role="dialog"` + labelled-by on dialogs are in place across `login/page.tsx`, `JoinForm`, `PassphraseGate`, `ConfirmDialog`, `CalendarSettingsSheet`, `DuplicateCalendarSheet`, `DayDetailSheet`, `FinalisedBanner`, and the header. `getByRole` / `getByLabel` should work end-to-end on the golden path without temporary testids. Verify per-spec as we write them; if any element still resists role+name selection, fix the component rather than reaching for `data-testid`.

## Open questions

- **Embedded Postgres vs Neon branches in CI** — start with embedded since `make test-integration` already uses it. Revisit if migrations-on-boot adds too much per-test overhead, or if we want true parallelism across specs.
- **Prod smoke tests** — should we also run a tiny read-only spec against prod post-deploy (health, homepage loads)? Probably yes eventually, but separate from this plan.
- **`E2E_TEST_MODE` guard placement** — where exactly does the refusal live (config layer? handler init?), and what stops it from being accidentally enabled in prod (CI assertion? Terraform check?).
- **Magic-link mailcatcher choice** — MailHog is the obvious pick but unmaintained; Mailpit is a maintained successor. Decide before Phase 3 step 6.
- **Frontend build mode in CI** — `pnpm dev` (fast boot, slow pages) or `pnpm build && pnpm start` (slow boot, prod-like). Lean prod build for CI, dev for local.

## Risks

- **CI runtime overhead** — Playwright install + boot both stacks ≈ 2–3 min per PR. Mitigate with caching (Playwright browsers, embedded Postgres binary, Go build cache).
- **Flakiness eroding trust** — if the suite goes red intermittently, people will start ignoring it. Budget time for fixing flakes; be willing to quarantine a spec rather than let it stay flaky.
- **Maintenance cost during UI churn** — selectors break when markup changes.
- **`E2E_TEST_MODE` foot-gun** — a misconfiguration that enables the test-login endpoint in prod is an account-takeover bug. Guard hard, alert if the flag is ever read in a prod-shaped environment.

## Log

- 2026-05-04 20:27 — Created plan
- 2026-05-04 20:35 — Refined: dropped mobile-viewport and overlap-display specs (not E2E-shaped); locked golden path to the actual API surface; added auth-in-tests strategy (test-login endpoint + one real magic-link spec); switched file extension to `.test.ts`; added `E2E_TEST_MODE` and mailcatcher open questions.
- 2026-05-04 20:42 — Added Selectors subsection (role+name first, testid as signal-for-missing-a11y) and a soft-dependency note tying spec ordering to `plans/ui/accessibility-pass.md` Phases 3–5.
- 2026-05-04 20:48 — Confirmed `plans/ui/accessibility-pass.md` is done; soft-dependency converted to a "resolved" note. No remaining a11y blockers for any spec.
- 2026-05-04 21:42 — Phase 1 landed: added `E2E_TEST_MODE` config (refused when `SECURE_COOKIES=true`), `POST /test/login` handler, `cmd/e2e-server` launcher (embedded Postgres + main server), `frontend/playwright.config.ts` with both webServers, `frontend/e2e/fixtures.ts`, `create-calendar.test.ts`, `make test-e2e`. Vitest excludes `e2e/`. First spec passes locally.
- 2026-05-04 22:10 — CI runtime tuning. First run on the PR was 2m33s, with `pnpm exec playwright install --with-deps chromium` taking 48s of that. Split into two steps: `playwright install chromium` gated on the browser cache (`steps.playwright-cache.outputs.cache-hit`), and `playwright install-deps chromium` running unconditionally (apt isn't cached by `actions/cache`). On warm cache that drops the install step from ~48s → ~25s.
- 2026-05-04 21:57 — Phases 2–4 landed. CI: new `e2e` job in `.github/workflows/ci.yml` with embedded-postgres + Playwright browser caches, uploads `playwright-report/` and `test-results/` on failure, gated to backend/frontend changes. Phase 4: wired `e2e` into `deploy-backend` / `deploy-frontend` `needs:` (success-or-skipped). Phase 3 specs: `join-anonymous`, `join-signed-in`, `submit-availability`, `finalise`, `claim-participant`, `magic-link-auth`. Magic-link uses an in-memory `CapturingEmailSender` wired automatically when E2E_TEST_MODE is on (no SMTP, no Mailpit/MailHog), exposed via `GET /test/magic-link?email=`. Resolves the open question on mailcatcher choice in this plan's favour: an in-process capture beats a real catcher because it stays hermetic and adds no Docker. Claim spec drives the endpoint explicitly rather than relying on the silent on-load claim, because the page reads `user` from closure once and races AuthProvider hydration on a fresh full-page navigation; the endpoint is the contract the silent path leans on. Full suite: 7 specs, 7 passing locally.
