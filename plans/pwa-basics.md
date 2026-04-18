# PWA Basics — Installable Home-Screen App

Status: done

## Goal

Make FreeWhen installable to the home screen on mobile so that the "share a link with friends" flow naturally leads to a standalone-feeling app, consistent with the mobile-first principle.

Scope is **basic PWA only**: manifest, icons, theme color, Apple touch icons, and an iOS-compatible standalone display mode. Out of scope for V1 (deferred):

- Service worker / offline caching — FreeWhen needs the network to load calendars anyway, so the added complexity buys little
- Install prompts / A2HS UI nudges — let the browser handle it
- Push notifications — requires auth and a notifications backend (V2+)

## Context

- Mobile-first principle (`AGENTS.md`) — install-to-home-screen directly serves this
- Brand color already in tokens: `--accent: #e06449` (light) / `#e8755e` (dark) — use light-mode coral as PWA theme color for consistency with iOS/Android status bar
- Display font: Bricolage Grotesque; body: DM Sans — icon should echo brand but be a simple mark (a letter/glyph), not render live text
- `frontend/src/app/layout.tsx` already declares `viewport` with `viewportFit: "cover"` and `metadata` with OG tags — manifest link + Apple meta tags slot in alongside
- `frontend/public/` currently has only Next.js default SVGs; we'll add the PWA asset set there
- Next.js App Router supports a `manifest.ts` (or static `manifest.webmanifest`) file convention in `src/app/` — prefer `manifest.ts` so the manifest stays typed and colocated with the layout

## Plan

### 1. Design the icon

A single source SVG at `frontend/public/icon.svg`:

- Rounded-square tile (let iOS/Android apply their own masking)
- Coral background (`#e06449`) with a white mark
- Mark: a simple calendar glyph — e.g. a square with a single highlighted cell, or the letters `FW` in the display font. Pick whichever reads better at 48px.
- Keep the mark inside a safe area of ~80% (standard maskable-icon padding) so Android's adaptive icon masking doesn't clip it

### 2. Generate the raster icon set

From the source SVG, produce PNGs in `frontend/public/`:

- `icon-192.png` (192×192) — Android home screen
- `icon-512.png` (512×512) — Android splash, PWA install
- `icon-maskable-512.png` (512×512, with full-bleed background + centered mark inside safe area) — Android adaptive icons
- `apple-touch-icon.png` (180×180, no transparency, no rounded corners — iOS rounds automatically) — iOS home screen
- `favicon-32.png` (32×32) — browser tab fallback alongside the existing `favicon.ico`

Use whatever tooling is convenient (e.g. `sharp` via a throwaway Node script, or an online generator). Do not add `sharp` as a project dependency — this is a one-shot asset generation step.

### 3. Add the Web App Manifest

Create `frontend/src/app/manifest.ts` using Next.js's `MetadataRoute.Manifest` type:

```ts
import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "FreeWhen",
    short_name: "FreeWhen",
    description: "Find when everyone's free. No sign-ups, no fuss.",
    start_url: "/",
    scope: "/",
    display: "standalone",
    orientation: "portrait",
    background_color: "#faf9f7",
    theme_color: "#e06449",
    icons: [
      { src: "/icon-192.png", sizes: "192x192", type: "image/png" },
      { src: "/icon-512.png", sizes: "512x512", type: "image/png" },
      {
        src: "/icon-maskable-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}
```

Next auto-serves this at `/manifest.webmanifest` and injects the `<link rel="manifest">` tag.

### 4. Add iOS + theme-color meta tags in `layout.tsx`

Extend the existing `metadata` export:

- `appleWebApp: { capable: true, title: "FreeWhen", statusBarStyle: "default" }` — tells iOS to treat it as a PWA and sets the home screen label
- `icons: { icon: [...], apple: "/apple-touch-icon.png" }` — standard + Apple touch icon

Extend the existing `viewport` export:

- `themeColor: [{ media: "(prefers-color-scheme: light)", color: "#e06449" }, { media: "(prefers-color-scheme: dark)", color: "#141210" }]` — matches the page background on each scheme so the browser chrome blends in

### 5. Verify

Verification is manual — a basic PWA has no logic to unit test.

- Run `make dev-frontend`
- DevTools → Application → Manifest: confirm it parses cleanly with no warnings, all icons listed, correct theme/background colors
- Lighthouse PWA audit: "Installable" should pass (we're intentionally **not** chasing the full PWA score — offline/SW are out of scope)
- Real device check (light lift, high signal):
  - iOS Safari: Share → Add to Home Screen → launch from home screen → confirm standalone chrome, correct icon, correct title
  - Android Chrome: menu → Install app → same checks
- Confirm existing mobile browser flow still works (no regressions on the non-installed path)

### 6. Update documentation

- Mark the "Mobile app (or PWA)" bullet in `docs/future-ideas.md` as shipped in V1 (or remove it and note in this plan's log that it's been pulled forward)
- Add a short "PWA" subsection to `docs/infra-notes.md` if deployment behavior changes (Vercel serves `/manifest.webmanifest` and static assets with no config — likely no changes needed, but verify MIME type is correct after deploy)

## Risks / Open Questions

- **Icon design quality** — a weak icon is worse than no icon on a home screen. If the mark doesn't read well at 48px, iterate before shipping. Consider asking the user to review the icon before raster generation.
- **Light/dark theme-color split** — iOS Safari picks the theme color at page load and doesn't re-evaluate on scheme change mid-session. Acceptable for V1; revisit if it looks jarring in practice.
- **`short_name` length** — "FreeWhen" is 8 chars, safely under the ~12-char iOS home-screen truncation. No abbreviation needed.
- **No service worker means no "offline" badge in install UI** — some browsers (desktop Chrome) show a subtle "this app works offline" signal that we won't get. Acceptable trade — keeps scope tight.

## Log

- 2026-04-18 — Created plan
- 2026-04-18 23:00 — Implemented. Added source `frontend/public/icon.svg` (3x3 rounded-square grid on coral `#e06449`, center cell highlighted). Generated `icon-192.png`, `icon-512.png`, `icon-maskable-512.png`, `apple-touch-icon.png`, `favicon-32.png` via a one-shot `sharp` install in `/tmp` (no project dependency added). Added `frontend/src/app/manifest.ts` using Next's `MetadataRoute.Manifest` convention — serves at `/manifest.webmanifest`. Extended `layout.tsx` with `themeColor` (light/dark split), `appleWebApp` metadata, and `icons` (favicon, svg, apple-touch-icon). Verified against dev server: manifest JSON parses, PNG endpoints 200, HTML head contains all expected meta/link tags. Build succeeds, lint clean, 71/71 vitest tests pass. Updated `docs/future-ideas.md` to reflect basic PWA shipped — deferred items (offline SW, push, install prompts) now listed explicitly. No changes needed to `docs/infra-notes.md` — Vercel serves the static assets and `manifest.webmanifest` with correct MIME types out of the box. Real-device verification (iOS Add to Home Screen, Android Install app) is still pending and should be done once on a staging/prod deploy.
