# Frontend open redirects and security headers

Status: done

## Goal

Close findings from a read-only security review:

- **M2 — open redirect on /login.** When the user is already signed in,
  `/login` calls `router.replace(returnUrl)` with the raw `?return_url=`
  query value, so `/login?return_url=https://evil.com` (or `//evil.com`,
  `/\evil.com`) bounces an authenticated user off-site.
- **L3 — magic-link return URL.** The review claimed `verifyMagicLink` reads
  `returnUrl` while the backend sends `return_url`. Verified: `request()`
  runs `toCamelCaseKeys` on every response, so the key *does* map correctly —
  not a bug. The callback page still navigates to the server value without a
  client-side check, so route it through the same safe helper (defence in
  depth) and pin the key mapping with a test. Also harden the backend
  `isRelativePath` to reject `\` and control characters.
- **L4 — no security headers.** Add baseline headers via `next.config.ts`.

## Plan

1. `src/lib/safeReturnPath.ts`: `safeReturnPath(raw)` returns `raw` only if
   it is a same-origin relative path — starts with `/`, second char not `/`
   or `\`, no `\` anywhere, no ASCII control chars or whitespace; otherwise
   `/`. Unit tests covering `//evil.com`, `/\evil.com`, `https://evil.com`,
   `/%2F%2Fevil.com` (allowed — stays a path), `javascript:`, tabs/newlines.
2. Use it in `login/page.tsx` (for the redirect *and* for the value passed
   to the magic-link / Google flows) and in the magic-link callback page.
3. Tests for login redirect behaviour (authenticated → safe path, hostile
   return_url → `/`).
4. Backend `isRelativePath`: also reject `\` and control chars / whitespace.
   Update `TestIsRelativePath` cases.
5. `next.config.ts` `headers()` for all routes:
   - `X-Frame-Options: DENY`
   - `Content-Security-Policy: frame-ancestors 'none'` (enforced — no risk)
   - `Referrer-Policy: strict-origin-when-cross-origin` (the magic-link
     callback already narrows itself to `no-referrer` via metadata)
   - `X-Content-Type-Options: nosniff`
   - `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=()`
     (leaves clipboard-write / web-share alone — ShareButton uses them)
   - Full CSP in **`Content-Security-Policy-Report-Only`**. Without nonces,
     Next.js App Router's inline hydration scripts plus our inline theme
     script need `'unsafe-inline'`, and the exact PostHog / Vercel origins
     are config-dependent, so enforcing now risks breaking the app. Policy
     allows `'self'`, `NEXT_PUBLIC_API_URL` (fetch + SSE), PostHog
     (`NEXT_PUBLIC_POSTHOG_HOST` + `*.posthog.com`), Vercel Speed Insights,
     `next/font` (self-hosted).
   - **Follow-up:** observe browser console / report violations in prod for a
     while, then flip to enforcing `Content-Security-Policy` (ideally with
     nonces via middleware to drop `'unsafe-inline'`).

## Log

- 2026-11-12 10:00 — Created plan; verified M2 and L4; L3 key mismatch is a
  false positive (camelCase conversion) but still adding client-side guard.
- 2026-11-12 10:40 — Added `safeReturnPath` + tests; wired into /login and
  the magic-link callback; login redirect tests; api test pinning the
  `return_url` → `returnUrl` mapping.
- 2026-11-12 10:45 — Backend `isRelativePath` now rejects `\`, whitespace and
  control chars; `TestIsRelativePath` cases updated (`/\evil.com` flipped to
  rejected). Go toolchain not available locally — backend test unverified
  here, relies on CI.
- 2026-11-12 10:50 — Security headers in `next.config.ts`; CSP shipped
  Report-Only (frame-ancestors enforced separately). Verified the config
  evaluates and emits the expected header values. Frontend: oxlint clean,
  oxfmt clean, vitest 299/299 on Node 24. Note: Vercel preview toolbar
  (vercel.live) will show as report-only violations — allow it or ignore it
  when flipping to enforce.
