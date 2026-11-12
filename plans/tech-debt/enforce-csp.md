# Enforce the Content-Security-Policy

Status: done

## Goal

`plans/bugfix/frontend-open-redirects-and-headers.md` shipped the full CSP as
`Content-Security-Policy-Report-Only` (only `frame-ancestors 'none'` is
enforced). Report-only stops nothing. Flip it to an enforced
`Content-Security-Policy` without breaking the app, keep it built from config
(no hard-coded prod domains), and add an e2e guard so a future change that
trips the policy fails the suite instead of silently breaking prod.

## Inventory — what the app loads

| Thing | Where it comes from | Directive |
| --- | --- | --- |
| Next App Router bootstrap / RSC flight (`self.__next_f.push(...)`) | inline `<script>` per page, content varies per request | `script-src 'unsafe-inline'` (or a nonce) |
| Colour-scheme init script (`COLOR_SCHEME_INIT_SCRIPT`) | static inline `<script dangerouslySetInnerHTML>` in `layout.tsx` | `script-src 'unsafe-inline'` |
| JS chunks, CSS | `/_next/static/*` | `'self'` |
| `next/font/google` (DM Sans, Fraunces, Instrument Serif) | self-hosted at build time under `/_next/static/media` | `font-src 'self'` |
| Inline styles (React `style=` props, Next font CSS vars) | inline | `style-src 'unsafe-inline'` |
| Noise texture in `globals.css` | `data:image/svg+xml` | `img-src data:` |
| Icons / manifest / favicon | `public/`, `app/manifest.ts` | `'self'` |
| `opengraph-image.tsx` | rendered server-side; fetches Google Fonts on the server, not in the browser | n/a |
| API (`NEXT_PUBLIC_API_URL`) fetch + realtime `EventSource` | API origin | `connect-src <api>` |
| `.ics` export | `<a href download>` to API origin — a navigation, not a fetch | n/a (no `navigate-to`) |
| Google sign-in / calendar connect | `window.location.href = <api>/auth/google...` — top-level navigation | n/a |
| Magic-link login + every other form | `onSubmit` + `fetch`; no native form POST anywhere | `form-action 'self'` |
| PostHog (`NEXT_PUBLIC_POSTHOG_KEY`/`_HOST`) | `posthog-js` is bundled and dynamically imported (self). It fetches remote config / lazy extensions (exception autocapture, web vitals) from the api host or `*-assets.i.posthog.com`, and posts to `api_host`. Surveys disabled. | `script-src`+`connect-src` `<posthog host>` + `https://*.posthog.com`, only when the key is set |
| Vercel Speed Insights | on Vercel: `/_vercel/speed-insights/script.js`, beacons to `/_vercel/speed-insights/vitals` (same origin). In `next dev` it loads `https://va.vercel-scripts.com/...script.debug.js` | `'self'`; dev server adds `va.vercel-scripts.com` |
| Workers / blob: | none in app code; PostHog's recorder can use a blob worker (recording isn't enabled, keep allowance) | `worker-src 'self' blob:` |
| Avatars | initials, no remote images | — |
| iframes / plugins | none | `object-src 'none'`, `frame-src` via `default-src 'self'` |

## Decision: `'unsafe-inline'` scripts, not nonces

A nonce policy (proxy generates a per-request nonce, `'strict-dynamic'`,
Next stamps it on its scripts) only works when every page is rendered per
request — Next reads the nonce from the request headers, so static
rendering, ISR and CDN caching of HTML are off for every route. The landing
page, `/login` and other shells are static today and the calendar page
server-render work (#78) depends on cacheable responses; giving those up for
the whole app is a real perf/cost regression.

Hashes don't work either: Next's flight scripts differ per page/request, and
adding any hash or nonce to `script-src` makes browsers ignore
`'unsafe-inline'`.

So: keep `'unsafe-inline'` in `script-src` (XSS protection here relies on
React escaping + no `dangerouslySetInnerHTML` on user data), and enforce
everything else tightly — script *origins* are pinned, `connect-src` is exact
origins only, `object-src 'none'`, `base-uri 'self'`, `form-action 'self'`,
`frame-ancestors 'none'`, `default-src 'self'`. That still blocks loading
attacker scripts from other origins, exfiltration via fetch/beacons to
arbitrary hosts, `<base>` hijacking, and form-POST exfiltration. Revisit
nonces if/when Next supports them for static routes.

## Dev server

React 19 / Next dev uses `eval()` for debugging features and Speed Insights
loads its debug script from `va.vercel-scripts.com`. `next.config.ts` exports
a function and adds `'unsafe-eval'` + that host only when
`phase === PHASE_DEVELOPMENT_SERVER`. This branches on what the *tool* is
doing (dev server vs build), not on a deploy environment label, so dev/prod
parity is preserved for every deployed environment.

## Vercel preview toolbar

`vercel.live` (comments toolbar on preview deploys) is not allowed — it
will be blocked on previews. Disable the toolbar in the project settings or
allow it later via config if wanted.

## Plan

1. `next.config.ts`: rename to `contentSecurityPolicy`, send as
   `Content-Security-Policy` (drop the separate frame-ancestors header and the
   Report-Only one). Export `(phase) => config`; dev phase adds the eval/debug
   allowances. Origins still from `NEXT_PUBLIC_API_URL` /
   `NEXT_PUBLIC_POSTHOG_HOST` / `NEXT_PUBLIC_POSTHOG_KEY`.
2. Playwright: run the frontend as `next build && next start` (the real
   production headers) instead of `next dev`.
3. `e2e/fixtures.ts`: auto fixture that records `securitypolicyviolation`
   events (init script) and CSP console errors, and fails the test if any
   occurred. All specs import from fixtures, so every flow is covered.
4. `e2e/csp.test.ts`: walk the flows not otherwise covered — landing, login,
   calendar page as owner with the rename + duplicate sheets, finalise →
   export buttons, `.ics` link.
5. Run the full e2e suite locally against `backend/cmd/e2e-server`; lint,
   unit tests; update `docs/onboarding.html` if it mentions the CSP.

## Log

- 2026-11-12 14:00 — Created plan; inventoried loads; chose `'unsafe-inline'`
  over nonces (static rendering).
- 2026-11-12 15:30 — `next.config.ts` now exports `(phase) => config` and
  sends the full policy as an enforced `Content-Security-Policy`; Report-Only
  and the separate frame-ancestors header removed. Verified headers: `next
  start` has no `'unsafe-eval'`; `next dev` adds `'unsafe-eval'` +
  `va.vercel-scripts.com`. `next build` still prerenders `/`, `/login`,
  `/calendars` etc. as static.
- 2026-11-12 15:40 — Playwright frontend server switched to `next build &&
  next start`; auto `cspGuard` fixture fails any test with a
  `securitypolicyviolation` / CSP console error; added `e2e/csp.test.ts`
  (header present, landing + login after idle, owner settings → rename,
  duplicate sheet, finalise → Google/.ics export with real download).
  Sanity-checked the guard with a throwaway spec injecting an external script
  + cross-origin fetch — it failed with the expected violations.
- 2026-11-12 15:55 — Local e2e against `backend/cmd/e2e-server` (embedded
  Postgres): 8 passed, 2 failed. Both failures are pre-existing backend authz
  behaviour hit by Playwright *API request contexts* (not the browser, so not
  CSP-related): `claim-participant` gets 403 on `/claim`, `finalise` gets a
  non-OK `PUT /availability` for another participant using the owner's
  session (owner-on-behalf writes were tightened in #74/#77). Zero CSP
  violations in any spec. Locally the webServer's `pnpm` resolved to a global
  pnpm 12 (corepack mismatch), so the frontend was started by hand with the
  same `next build && next start` command and reused. oxlint clean, vitest
  411/411.
