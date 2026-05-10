# Identity provider abstraction

Status: done

> **Decision (2026-05-11):** worth doing now, not waiting for the trigger.
> Mirrors the calprovider call: the design is already nailed down (see
> "Sketched interface" below), the change is small (one Go interface, the
> Google-shaped bits of `handler/auth.go` moved behind it, a registry plumbed
> into `NewAuth`), and the audit below shows the leak surface is small enough
> that the refactor is cheaper now than after more handler edits land. Not
> flipped to in-progress yet — pick up when someone has a slot.

## Goal

Put an interface seam between the sign-in handler and the Google-specific
OAuth/userinfo code in `internal/handler/auth.go`, so when a second identity
provider lands — Microsoft, Apple, whatever — we don't rediscover the same
seams under deadline pressure. Sibling to the calendar plan: structurally
identical, kept separate on purpose (different token lifetimes, scope models,
failure modes).

## Why we want this

- **The contract is the hard part, not the impl.** Identity payload shape
  (`sub` vs `oid` vs Apple's opaque `sub`), email-verification semantics,
  ID-token-vs-userinfo-endpoint trade-offs, "private relay" emails (Apple) —
  these are worth thinking through once. Designing them while Google is the
  only impl bakes Google's quirks into the handler.
- **The first impl already leaks.** `googleUserInfo` struct, the Google
  userinfo JSON shape, and the `https://...googleapis.com/oauth2/v3/userinfo`
  URL are all visible in the handler. Left alone, "add Microsoft" becomes
  "rewrite the handler" rather than "drop in a new impl."
- **Testability.** An interface gives the auth handler a fake for free. The
  current `WithOAuthEndpoint` / `WithUserInfoURL` options are a partial
  version of this — they exist *because* the seam is missing.
- **Symmetry with calprovider.** The calendar refactor landed
  (`internal/calprovider/`). Identity should follow the same pattern so the
  codebase has one shape, not two.

## Why not now

- **Single concrete OAuth impl today.** A one-impl interface is just
  indirection, and the second impl is what tells you what actually varies.
- **Magic links take the pressure off.** "I don't have a Google account" is
  already served by the email path, so a second OAuth provider is less
  urgent than it would be otherwise.

## Trigger

Promote to in-progress when **either** happens:

1. A second identity provider is on the roadmap (Microsoft, Apple, GitHub).
2. The Google sign-in path grows enough that testing it without faking
   Google's userinfo endpoint becomes painful — the fake-for-tests argument
   alone justifies the seam.

## Where Google currently leaks

- **`internal/handler/auth.go`** — the handler holds an `*oauth2.Config`
  directly and calls `h.oauth.Exchange` in `GoogleCallback`. OAuth is
  provider-shaped (endpoint URLs, scope strings, redirect URL).
- **`defaultGoogleUserInfoURL`** — hardcoded constant in the handler file.
  Microsoft's is `https://graph.microsoft.com/oidc/userinfo` (or `/me`
  against Graph); Apple doesn't expose a userinfo endpoint at all and
  requires decoding the ID token instead.
- **`googleUserInfo` struct + JSON shape** — `{sub, email, name}` baked
  into the handler. Microsoft returns `{sub, email, name}` from the OIDC
  userinfo endpoint (close enough) but Graph's `/me` returns
  `{id, mail, userPrincipalName, displayName}` — different field names,
  and `mail` can be empty for personal accounts.
- **Scopes** — `["openid", "email", "profile"]` hardcoded at handler
  construction. Microsoft uses the same OIDC strings; Apple uses
  `["name", "email"]` and the names only come back on first sign-in. Each
  provider has its own minimum-privilege scope set.
- **Config fields** — `GoogleClientID` / `GoogleClientSecret` /
  `GoogleRedirectURL` are single fields. A second provider needs parallel
  fields and a second callback route. Today there's no place to put that.
- **Cookie path** — `/auth/google/callback` is hardcoded for the nonce
  cookie (set + clear). Should derive from `provider.Name()`.
- **`findOrCreateUser`** — passes the literal string `"google"` into
  `FindUserByProviderID` and `CreateUser`. Provider name should come from
  the provider itself.
- **Routes** — `/auth/google` and `/auth/google/callback` are hardcoded in
  `server.go`. Should become `/auth/{provider}` and
  `/auth/{provider}/callback`, looking up via the registry.
- **OAuth state encoding** (`oauthState` struct) — provider-agnostic
  already. No work needed.

## Sketched interface

Settled enough to write down. One real op (resolve identity), plus the
OAuth surface to drive the round-trip:

```go
type Provider interface {
    Name() string                                  // "google", "microsoft", "apple"
    OAuthConfig() *oauth2.Config                   // for the sign-in scope
    Identify(ctx context.Context, token *oauth2.Token) (Identity, error)
}

type Identity struct {
    Sub   string // provider's stable, opaque user id
    Email string // canonical lowercase; "" if provider withholds
    Name  string // display name; "" if not provided
}
```

Notes:

- `Identify` takes `*oauth2.Token` rather than just an access token string.
  Apple returns identity claims in the ID token (`token.Extra("id_token")`),
  not from a userinfo endpoint — passing the whole token lets each provider
  pick its own source without leaking that choice through the interface.
- `OAuthConfig()` returns `nil` when the provider's credentials aren't
  configured for this environment (matches calprovider's behaviour for
  dev). Routes for that provider gate on `OAuthConfig() != nil`.
- Email canonicalisation (`normalizeEmail`) stays in the handler. Provider
  impls return whatever the upstream gives; the handler lowercases before
  any store call. Audit F15 stands.
- `Sub` is opaque per-provider. Two providers can mint the same `sub`
  string; `FindUserByProviderID(provider, sub)` already keys on the pair.

## Settled invariants

- **No token persistence.** Identity tokens are used once at sign-in. The
  session cookie is the only thing that survives the request. The interface
  must not grow `RefreshToken`/`StoreToken`/etc.
- **OAuth happens in the handler, not the provider.** The provider
  *describes* its OAuth config; the handler runs `Exchange` and passes the
  resulting token to `Identify`. Keeps provider impls testable without
  also faking the token endpoint.
- **One identity per user row.** Today the schema is one
  `(provider, provider_id)` pair per user. Multi-identity (Google + Microsoft
  linked to the same account) is a schema change and is out of scope —
  flagged in open questions, not promoted to the interface.
- **Magic links are not a `Provider`.** The email path stays its own
  handler. Forcing it through the OAuth-shaped interface would be the wrong
  abstraction (no `oauth2.Config`, different lifecycle, different attack
  surface). The F6 timing-uniformity invariant in `SendMagicLink` is the
  reason — don't entangle it with branching that depends on provider state.

## Open questions (deferred until trigger fires)

Not blockers — captured so they aren't rediscovered under deadline pressure:

1. **Email verification.** Google sets `email_verified: true` on its
   payload; we don't currently check it. Microsoft work accounts are
   implicitly verified; personal accounts vary. Apple's private-relay
   emails are verified-but-aliased. Do we trust the provider, re-verify
   via magic link, or refuse private-relay aliases? Designing the policy
   before seeing Apple's actual shape is guesswork.
2. **Account linking.** Today, signing in with Google for an email that
   already has a magic-link account merges by email (`findOrCreateUser`
   falls through to `FindUserByEmail`). Should a *second* OAuth identity
   (Google + Microsoft, both for the same email) attach to the same user
   row, or stay separate? Schema change either way.
3. **PKCE.** Confidential clients don't strictly need PKCE. Apple
   recommends it; Microsoft requires it for SPA flows but not for
   server-side. Worth a one-time decision when Apple lands rather than
   per-provider.
4. **Provider selection in the UI.** Today the frontend has a single
   "Sign in with Google" button. Multi-provider needs either a chooser or
   a "Continue with…" pattern. Frontend work, not Go's problem.
5. **ID-token verification.** If a provider's `Identify` reads claims
   from the ID token (Apple), the impl must verify the JWT signature
   against the provider's JWKS. Spec'd per-provider, not on the interface.

## Scope (deliberately deferred)

- Exact API shape of the provider interface (see Q1–Q5 above).
- How `users` schema accommodates multi-identity (see Q2).
- Frontend changes: provider chooser UI, per-provider button copy,
  routing for a second callback path.

Designed when the trigger fires. The audit above means whoever picks this
up isn't starting from a blank page.

## Non-goals

- **Not unifying magic links with OAuth.** Email is a separate auth
  method; forcing it through `Provider` is the wrong abstraction. See the
  settled invariant above.
- **Not a plugin system.** Two or three hardcoded providers compiled in is
  fine. No dynamic discovery.
- **Not changing the no-token-persistence property.** Whatever interface
  emerges, "identity tokens are used once and discarded" stays a system
  invariant.
- **Not changing session/cookie semantics.** Session lifetime, F2 absolute
  expiry, F11 concurrent-session cap, F12 LogoutAll — all stay as-is. The
  seam is upstream of session creation.

## Sketched migration

The shape, not the diff. Mirrors what landed for calprovider so the
eventual PR is mechanical:

1. New `internal/idprovider/` package:
   - `provider.go` — `Provider` interface, `Identity` struct, `Registry`
     (copy `calprovider.Registry` almost verbatim).
   - `google.go` — moves the Google-shaped bits of `auth.go` (scope list,
     userinfo URL, `googleUserInfo` struct, `oauth2.Config` construction)
     behind the interface. `NewGoogle(clientID, clientSecret, redirectURL,
     opts...)` parallel to `calprovider.NewGoogle`.
   - `google_test.go` — ports the existing `auth_google_test.go` userinfo
     tests verbatim, plus registry-lookup and nil-safety coverage.
2. `AuthHandler` changes:
   - Drop `oauth *oauth2.Config` and `userInfoURL` fields. Add
     `idProviders *idprovider.Registry`.
   - `GoogleLogin` / `GoogleCallback` become `OAuthLogin` /
     `OAuthCallback`, look up provider via `r.PathValue("provider")`, and
     return 404 for unknown providers. Cookie path derives from
     `provider.Name()`.
   - `findOrCreateUser` takes `Identity` + `providerName string` instead
     of `googleUserInfo`. The literal `"google"` goes away.
3. Routes in `server.go`:
   - `/auth/google` → `/auth/{provider}`
   - `/auth/google/callback` → `/auth/{provider}/callback`
   - `route_coverage_test.go` updated to match.
4. Frontend: no change required for this PR. Verified
   2026-05-11: only one frontend reference to the sign-in route
   (`frontend/src/lib/api.ts:381` builds `/auth/google?return_url=...`),
   which matches `/auth/{provider}` cleanly. Adding a chooser is follow-up
   work for whoever lands the second provider.

## Relationship to the calendar plan

Structurally identical to `calendar-provider-abstraction.md` (now done),
but kept separate on purpose:

- Identity sign-in tokens drive a *persisted session*. Calendar tokens are
  single-use and discarded immediately. The lifetime difference is the
  whole reason the no-token-persistence invariant lives on each interface
  separately.
- Scope models differ — identity uses OIDC strings (`openid email
  profile`), calendar uses provider-specific scopes
  (`calendar.freebusy`, `Calendars.Read`).
- Failure modes differ — identity failure means "can't sign in" (rare,
  user-visible at the top of the funnel). Calendar failure means "can't
  read busy times" (recoverable, mid-flow).

A shared OAuth substrate (e.g. a common `oauth2.Config` helper) is
plausible once both seams exist, but not assumed. Decide if/when it
actually pays for itself.

## Log

- 2026-05-10 — Created placeholder. Sibling to the calendar provider plan;
  flags the same one-impl lock-in for the sign-in path so the audit isn't
  rediscovered later.
- 2026-05-11 — Verified no frontend coupling blocks the route change:
  `frontend/src/lib/api.ts:381` is the only sign-in route reference and
  hits `/auth/google?return_url=...`, which matches `/auth/{provider}`
  without any frontend edit.
- 2026-05-11 — Landed. New `internal/idprovider/` package with `Provider`
  interface (`Name`, `OAuthConfig`, `Identify(ctx, *oauth2.Token) → Identity`)
  and `Registry` mirroring calprovider. `Google` impl owns the OIDC userinfo
  call and scope list. `AuthHandler` no longer holds an `*oauth2.Config` or
  userinfo URL — drops `WithOAuthEndpoint` / `WithUserInfoURL` options too;
  test seams now live on the provider. `GoogleLogin`/`GoogleCallback` →
  `OAuthLogin`/`OAuthCallback`, resolving via `r.PathValue("provider")` and
  returning 404 for unknown providers, 503 for unconfigured. Nonce cookie
  path derives from `provider.Name()`. `findOrCreateUser` takes
  `(providerName, Identity)` — the literal `"google"` is gone. Routes
  `GET /auth/google[/callback]` → `GET /auth/{provider}[/callback]`;
  route-coverage allow-list updated. Userinfo non-2xx now surfaces as 500
  (previously silently proceeded with zero values); test updated. No
  frontend or migration changes needed. `make lint`, `make test`, and
  `make test-integration` all green.
- 2026-05-11 — Expanded to a real plan, mirroring the structure that
  landed for calprovider. Concrete audit of where Google leaks (handler
  fields, hardcoded userinfo URL/struct, hardcoded scopes/cookie path,
  string "google" in `findOrCreateUser`, hardcoded routes). Sketched the
  `Provider` interface (`Name`, `OAuthConfig`, `Identify(ctx, *oauth2.Token)
  → Identity`) and recorded why `Identify` takes the whole token rather
  than just an access string (Apple ID-token claims). Settled invariants:
  no token persistence, OAuth runs in the handler, one identity per user
  row today, magic links stay out of the interface. Open questions
  parked: email-verification policy, account linking, PKCE, provider
  chooser UI, ID-token verification. Sketched the migration so the
  eventual PR is mechanical. Decision: pull this forward rather than
  wait for the trigger — same reasoning as calprovider (small change,
  design done, cheaper at one impl). Status left as `pending`; flip when
  someone picks it up.
