# Feature Flags

Status: in-progress

## Goal

Adopt a feature-flag / feature-management service so we can ship code to prod gated behind a flag, roll features out to a subset of users, and kill-switch regressions without redeploying. Today every merge to main is "on" the moment Cloud Run rotates, which is fine at friends-and-family scale but rules out:

- Shipping half-finished work behind a flag (trunk-based development with long-lived features).
- Gradual rollouts (1% → 10% → 100%) for risky changes like the availability-writing path.
- Per-user experiments (e.g. copy tests on the share CTA, a different empty-state for new calendars).
- Fast rollback without a Vercel/Cloud Run redeploy when something breaks.

The observability stack already landed PostHog; we should decide whether to reuse it for flags or adopt a dedicated vendor, and commit to an SDK shape before we start sprinkling `if (flag)` across the codebase.

## Context

- Frontend is Next.js on Vercel; backend is Go on Cloud Run. Any chosen vendor needs SDKs for both — or at least a REST endpoint we can wrap.
- We already follow "config over environment detection" (AGENTS.md). A flag service is the natural next step: instead of config toggles read at startup, we get runtime toggles with targeting rules.
- PostHog is already initialised in `frontend/src/lib/analytics.ts`, gated on `NEXT_PUBLIC_POSTHOG_KEY`. Its feature-flag product is included in the same free tier.
- We have no paid infra beyond domains — free tier is a hard requirement for v1.

## Options

| Vendor | Free tier (verify at adoption time) | Go SDK | JS SDK | Experimentation | Notes |
|--------|--------|--------|--------|-----------------|-------|
| **PostHog** | 1M flag requests/mo, bundled with analytics free tier | Yes | Yes (already installed) | Yes | Zero new vendor. EU region already chosen. One dashboard. |
| **Statsig** | 1M events/mo, unlimited flags | Yes | Yes | Strong (built for experimentation) | Best stats engine of the bunch. Separate vendor to manage. |
| **GrowthBook** | 5 users, unlimited flags on hosted free tier; self-host is free | Yes | Yes | Yes | Open-source, SQL-native experimentation (reads from our Postgres/warehouse). More DIY. |
| **Unleash** | Self-host free; hosted dev tier free, paid beyond | Yes | Yes | Limited | Purist flags, no built-in analytics. Adds ops if self-hosted. |
| **Flagsmith** | 50k requests/mo hosted, self-host free | Yes | Yes | Limited | Smaller ecosystem. |
| **LaunchDarkly** | 14-day trial only | Yes | Yes | Yes | Ruled out — no permanent free tier. |

## Recommendation

**Use PostHog feature flags.** Reasoning:

- Zero new vendor, zero new SDK on the frontend — `posthog-js` is already imported and initialised. Flags are a method call away (`posthog.isFeatureEnabled('flag-key')`).
- Targeting rules can use the same `identify()` properties we're already setting (user id, email) plus anonymous distinct-ids for not-signed-in joiners.
- Free tier comfortably covers current and foreseeable scale (1M flag requests/mo).
- One dashboard for analytics + flags means a single place to reason about a rollout ("flipped flag X to 10%, signups dropped — revert").
- If experimentation outgrows PostHog's stats engine later, we can layer Statsig on top without undoing the flag plumbing.

The cost of this choice is lock-in to PostHog's flag API. Mitigated by the abstraction in Phase 1 below — we expose our own `isEnabled(key, fallback)` helper, not raw `posthog.*` calls at use sites.

## Plan

### Phase 1 — Frontend flag helper

- Extend `frontend/src/lib/analytics.ts` (or carve out `src/lib/flags.ts` that imports from it) with a narrow API:
  - `isEnabled(key: string, fallback = false): boolean`
  - `getVariant(key: string, fallback: string | null = null): string | null` for multivariate flags
  - `onFlagsLoaded(cb: () => void): void` for components that need to wait for hydration
- When `NEXT_PUBLIC_POSTHOG_KEY` is unset (local dev without flags), `isEnabled` returns the `fallback` — consistent with how analytics no-ops today.
- Verify flag-evaluation APIs against the current `posthog-js` docs via context7 before locking in method names — bootstrap vs. async evaluation has moved before.
- Bootstrap flags on the initial render (PostHog's `bootstrap` option) so SSR output matches client output and we don't flash the fallback state.

### Phase 2 — Backend flag helper

- Add `internal/flags/` with an `Evaluator` interface: `Enabled(ctx, key, userID string) bool`, `Variant(ctx, key, userID string) string`.
- Implement with PostHog's Go SDK (`github.com/posthog/posthog-go`), reading `POSTHOG_KEY` and `POSTHOG_HOST` from config (new entries in `internal/config/`). Local polling of flag definitions keeps evaluation in-process and fast (no per-request network hop).
- When `POSTHOG_KEY` is unset, wire a `NoopEvaluator` that always returns fallback — same 12-factor pattern as the frontend.
- Expose the evaluator through the handler struct the same way `store` is threaded today, so handlers call `h.flags.Enabled(ctx, "some-key", userID)` rather than reaching for a global.

### Phase 3 — First real flag + rollout drill

Pick one genuinely low-stakes change to exercise the end-to-end flow before anyone relies on it. Candidate: the "Accept all suggestions" button from `plans/v2/t3/calendar-integrations.md` when that lands, or — sooner — gating a small copy change on the share sheet.

- Create the flag in PostHog with a rollout percentage slider.
- Wrap the feature in `isEnabled('share-cta-v2')` on the frontend.
- Roll 0% → 10% → 100% over a few days, watching the PostHog events dashboard for regressions.
- Once at 100% and stable for a week, **delete the flag and the branch of code it gated.** Document this as the expected lifecycle — flags are not permanent configuration.

### Phase 4 — Docs and conventions

- Update `docs/stack.md`: extend the PostHog section to cover feature flags alongside analytics (same SDK, same gating env vars, mention the backend evaluator). Only land this once Phases 1–2 are merged — `stack.md` describes what's actually in the tree, not what's planned.
- Update `AGENTS.md` Principles with a line about flags: *long-lived flags are a smell; flags gate in-flight work and are removed once the feature is fully rolled out.*
- Add a short section to `docs/infra-notes.md` covering how to create a flag in PostHog, naming conventions (`kebab-case`, prefix with area — `calendar-`, `auth-`, `share-`), and the retirement checklist.

## Non-goals

- **A/B testing / experimentation v1.** Flags first, experiments later. PostHog supports both; we'll layer experimentation on once we have a flag we actually want to run a stats test on.
- **Server-driven flag configs checked into git** (e.g. Unleash's approach). PostHog's dashboard is the source of truth; we accept that trade-off for zero ops burden.
- **Flag-as-config for infra settings** like `DATABASE_URL` or `CORS_ORIGIN`. Those stay as env vars per 12-factor — flags are for runtime user-facing behaviour, not startup configuration.
- **Self-hosting PostHog.** Same reasoning as in `plans/v2/t2/observability-and-analytics.md` — cloud free tier is enough and avoids ops.

## Open questions

- Do we want flag evaluations to also fire as PostHog events (`$feature_flag_called`) so we can correlate flag state with funnels? Default is yes — PostHog does this automatically. Worth confirming it doesn't chew through the 1M/mo event allowance on chatty flags.
- For anonymous joiners, what's the distinct-id? Likely the existing session/participant identifier — needs a quick look at `AuthProvider` and the participant session flow.

## Log

- 2026-04-22 — Created plan
- 2026-04-22 — Clarified Phase 4: docs updates to `stack.md`/`AGENTS.md`/`infra-notes.md` are part of implementation, not pre-empted alongside the plan
- 2026-04-23 21:55 — Phase 1: added `frontend/src/lib/flags.ts` with `isEnabled`/`getVariant`/`onFlagsLoaded`, no-op when `NEXT_PUBLIC_POSTHOG_KEY` is unset
- 2026-04-23 21:55 — Phase 2: added `internal/flags/` (Evaluator interface, Noop, PostHogEvaluator using `posthog-go`). New config entries `POSTHOG_KEY`/`POSTHOG_HOST`/`POSTHOG_PERSONAL_API_KEY`. Evaluator threaded onto Handler via `WithFlags(...)` (chosen over changing `handler.New` to keep the 20+ test call sites untouched)
- 2026-04-23 21:55 — Phase 3 (rollout drill) intentionally deferred — it's an operator exercise against the PostHog dashboard with a real flag, not a code change. The infrastructure for it is in place
- 2026-04-23 21:56 — Phase 4: updated `docs/stack.md`, `docs/infra-notes.md` (env vars + flag conventions block), and `AGENTS.md` (flags-are-temporary principle)
