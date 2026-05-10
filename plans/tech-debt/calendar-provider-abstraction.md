# Calendar provider abstraction

Status: done

> **Decision (2026-05-10):** worth doing now, not waiting for the trigger.
> The design is already nailed down (see "Sketched interface" below), the
> change is small (one Go interface, ~90 lines of `freebusy.go` and the
> deep-link builder moved behind it, handler takes `CalendarProvider`
> instead of `FreeBusyFetcher`), and fixing the Google-leaks-into-handler
> smell is cheaper now than after more handler edits accumulate. Not
> flipped to in-progress yet — pick up when someone has a slot.

## Goal

Pin down *now* the case for putting an interface seam between the calendar
integration handler and the Google-specific code in `internal/google/`, so the
day we add a second provider (Microsoft Graph being the obvious next one) the
design conversation has already happened and we don't end up rewriting the
handler under deadline pressure.

This plan is deliberately a placeholder — no API shape, no migration steps. It
exists to record the **decision and the reasoning**, not the implementation.

## Why we want this

- **The contract is the hard part, not the impl.** Token lifecycle, error
  taxonomy, timezone handling, partial-result semantics — these are things
  worth thinking through once. Designing them while Google is the only impl
  bakes Google's quirks into the handler; designing them as a contract forces
  the right questions up front.
- **The first impl will leak otherwise.** Google's FreeBusy response shape
  (calendar IDs, primary vs secondary, per-calendar `errors[]`) is already
  visible in the handler. Left alone, "add Microsoft" becomes "rewrite the
  handler" rather than "drop in a new impl."
- **Testability.** An interface gives us a fake for free. The second consumer
  is the test suite, not necessarily a second provider — and that alone often
  justifies the seam.
- **Asymmetric to ICS.** ICS is already the universal abstraction (it's a
  standards-body interface). The Google import path has no such fallback —
  every non-Google provider needs bespoke OAuth + API plumbing. That's where
  the abstraction earns its keep.

## Why not now

- **Single concrete impl today.** A one-impl interface is just indirection,
  and the second impl is what tells you what actually varies. Implementing
  the seam before Microsoft Graph (or equivalent) risks designing for
  imagined symmetry rather than real differences.
- **No active driver.** Nobody is asking for a second freebusy source this
  quarter. Doing the refactor now competes with shipping features.

## Trigger

Promote this to in-progress when **either** happens:

1. A second freebusy source is on the roadmap (Microsoft Graph, Apple/CalDAV,
   anything).
2. The Google integration grows enough that testing it without hitting real
   Google APIs becomes painful — at which point the fake-for-tests argument
   alone justifies the seam, second provider or not.

## Where Google currently leaks

A quick audit of what's Google-shaped today, so the eventual refactor has a
checklist of friction points rather than a vague "extract an interface":

- **`internal/google/freebusy.go`** — already isolated, but its return type
  (`[]availability.TimeRange`) is the de-facto wire format the rest of the
  system has standardised on. Good: that means a provider interface returning
  `[]availability.TimeRange` is plausible. Bad: the function silently
  collapses Google's `calendars["primary"]` / `errors[]` shape into a single
  `(busy, err)` pair, which means provider-specific failure modes
  (e.g. partial-calendar errors, scope downgrade, secondary calendars) are
  already being flattened. Microsoft Graph returns a different shape
  (`scheduleItems` with `status`) and won't fit cleanly into the same
  flattening logic.
- **`internal/handler/calendar_integration.go`** — the handler holds an
  `*oauth2.Config` directly and calls `h.oauth.Exchange` in `Callback`. OAuth
  is provider-shaped (endpoint URLs, scope strings, redirect URL, the
  `include_granted_scopes` quirk is Google-only). A second provider needs its
  own `oauth2.Config` with different scopes and a different callback path —
  today there's no place to put that.
- **Scope string** — `https://www.googleapis.com/auth/calendar.freebusy` is
  hardcoded at handler-construction time. Microsoft's equivalent is
  `Calendars.Read` (or `Calendars.ReadBasic.All` for free/busy without event
  content). Each provider has its own minimum-privilege scope and the
  interstitial copy needs to match.
- **Redirect URL config** — `GoogleCalendarRedirectURL` is one config field.
  A second provider needs a parallel field (`MicrosoftCalendarRedirectURL`)
  and routing for a second callback path (`/auth/microsoft/calendar/callback`
  or similar). The route registration in `internal/server/` will need to
  multiplex.
- **Single-use suggestion cookie** — provider-agnostic already. No work
  needed here.
- **Consent ack** — `SetCalendarImportConsentAck` records a single boolean.
  Whether that's per-provider or global is a product decision, not a
  technical one — but worth flagging so it doesn't get implicitly answered
  by whoever does the refactor.
- **"Add to Google Calendar" deep link** (`handler/export.go:54`,
  `frontend/src/components/ExportButtons/ExportButtons.tsx`) — builds a
  `calendar.google.com/calendar/render?...` URL and redirects. *Not* an
  API call, no OAuth, no write scope. Microsoft has an equivalent
  (`outlook.live.com/calendar/0/deeplink/compose?...`). Both providers
  need a URL-builder method on the interface; neither needs token
  plumbing for this path.

## Sketched interface

Settled enough to write down. Two methods covering the two real
provider-shaped operations today (free/busy import and "add event" deep
link), plus enough OAuth surface to plumb the import scope:

```go
type CalendarProvider interface {
    Name() string                                              // "google", "microsoft"
    OAuthConfig() *oauth2.Config                               // for FreeBusy scope only
    FreeBusy(ctx context.Context, token string,
        start, end time.Time, tz string) ([]availability.TimeRange, error)
    AddEventURL(event ExportEvent) string                      // deep link, no API call
}

type ExportEvent struct {
    Title  string
    Date   time.Time
    AllDay bool
}
```

Notes:

- `OAuthConfig()` exposes the `oauth2.Config` directly rather than wrapping
  it. Cheaper, and the handler already speaks `oauth2`. Revisit if a
  provider needs auth that doesn't fit `oauth2.Config` cleanly.
- `AddEventURL` is a pure function — no token, no context. Both Google's
  render URL and Outlook's compose URL are fully described by event fields.
- `Interstitial()` is intentionally *not* on this interface. Interstitial
  copy lives in the frontend (`ImportFromGoogle.tsx`), not the backend.
  When a second provider lands, either the frontend gets a
  `switch provider` over copy, or copy moves into `next-intl`/similar.
  Either way, it's not the Go interface's problem — **assuming
  interstitials differ only in cosmetic ways (provider name, icon).** If
  Microsoft's consent flow turns out to need materially different copy
  (e.g. admin-consent warnings for work accounts, different legal
  language, scope explanations the frontend shouldn't have to know), the
  right answer flips: a structured `ConsentInfo` (or similar) on the Go
  interface so the provider owns those details. Revisit when we actually
  look at Microsoft's requirements.

## Settled invariants

- **No token persistence.** The interface must not grow `RefreshToken()`,
  `StoreToken()`, or any method that implies persisted credentials. Every
  import stays a fresh OAuth round-trip.
- **Free/busy read + event deep-link only.** Writing events back via API,
  syncing, webhooks — all out of scope (see Non-goals).
- **OAuth happens in the handler, not the provider.** The provider
  *describes* its OAuth config; the handler runs the exchange. Keeps the
  provider impls pure-ish and testable.

## Open questions (deferred until trigger fires)

Not blockers — captured so they aren't rediscovered under deadline pressure:

1. **Error taxonomy.** Today `FreeBusy` returns plain errors. A second
   provider forces the question: do we need typed errors for
   "token expired / scope downgraded / rate limited / transient" so the
   handler can render appropriate UX? Probably yes — but designing the enum
   before seeing Microsoft's failure modes is guesswork.
2. **Provider selection.** Per-user (`users.calendar_provider`) vs
   per-calendar vs "user picks each time"? Probably per-user, but it
   interacts with whether one user can have both Google and Microsoft
   calendars linked.
3. **Testing strategy.** The seam should make the *handler* testable
   without an HTTP fake — i.e. inject a `CalendarProvider` whose `FreeBusy`
   returns canned data, rather than today's `FreeBusyFetcher` function
   field. The existing `WithFreeBusyFetcher` option is a partial version of
   this and probably gets absorbed into the new interface.

## Scope (deliberately deferred)

- Exact API shape of the provider interface (see Q1 above).
- How OAuth tokens / scopes are plumbed per-provider (see Q4).
- How `users.calendar_provider` (or equivalent) selects an impl (see Q3).
- Migration steps for existing Google integration code.
- Frontend changes: provider picker UI, per-provider interstitial copy,
  routing for a second callback path.

All of the above gets designed when the trigger fires. Writing it now would
just be speculative — but the audit above means whoever picks this up isn't
starting from a blank page.

## Non-goals

- **Not a generic "calendar abstraction".** This is specifically about the
  free/busy import path. Reading/writing events, syncing, webhooks — none of
  that is in scope. ICS export remains its own thing.
- **Not a plugin system.** Two or three hardcoded providers compiled in is
  fine. We are not building dynamic provider discovery.
- **Not changing the no-token-persistence property.** Whatever interface
  emerges, "tokens are used once and discarded" stays a system invariant.

## Log

- 2026-05-10 22:50 — Created plan. Lightweight, intentionally — captures the
  decision to do this once we have a second provider, not the specifics.
- 2026-05-10 23:10 — Expanded with concrete audit of where Google leaks
  today (freebusy response flattening, hardcoded scope, single OAuth config,
  single redirect URL config) and open questions to answer when triggered
  (interface shape, error taxonomy, provider selection, token storage,
  consent copy, testing). Added non-goals to bound the eventual refactor.
- 2026-05-10 23:55 — Decided to pull this forward rather than wait for the
  trigger. Reasoning: design is already done, refactor is small, and
  fixing the Google-leaks-into-handler smell is cheaper at one impl than
  after more handler edits land. Status left as `pending` (not yet
  in-progress) — flip when someone picks it up.
- 2026-05-10 23:40 — Added caveat to the interstitial decision: keep copy
  in the frontend *unless* a second provider's consent flow needs
  materially different copy (admin-consent warnings, distinct legal
  language), in which case a structured `ConsentInfo` on the Go interface
  is the right answer. Revisit when looking at Microsoft's requirements.
- 2026-05-11 00:10 — Implemented. New `internal/calprovider/` package
  defines `Provider` interface (`Name`, `OAuthConfig`, `FreeBusy`,
  `AddEventURL`), `ExportEvent`, and a `Registry` (`map[name]Provider`).
  Google moved out of `internal/google/` (deleted) into
  `calprovider/google.go` as the first impl — freebusy tests ported
  verbatim. `CalendarIntegrationHandler` lost its `*oauth2.Config` and
  `FreeBusyFetcher` fields; routes are now provider-agnostic
  (`/auth/{provider}/calendar/{connect,callback,consent-ack}` and
  `/calendars/{id}/export/{provider}`) and the handler does
  `registry.Get(r.PathValue("provider"))` so adding Apple/Microsoft is
  one new file + one registry entry, no handler/route changes. Cookie
  paths derive from `provider.Name()`. `ExportGoogle` became generic
  `ExportToProvider` using `provider.AddEventURL`. Export deep-link
  keeps working without OAuth creds (registry always carries Google;
  OAuth surface gates on `OAuthConfig() != nil`).
- 2026-05-10 23:30 — Corrected scope: there are *two* provider-shaped ops
  today, not one — free/busy import (real API call) and "Add to Google
  Calendar" deep link (URL builder, no API). Promoted Q1/Q4/Q5 from open to
  settled: sketched the `CalendarProvider` interface (`Name`,
  `OAuthConfig`, `FreeBusy`, `AddEventURL`), recorded no-token-persistence
  as an interface-level invariant, and decided `Interstitial()` doesn't
  belong on the Go interface (copy lives in the frontend). Q2/Q3/Q6 stay
  deferred.
