# Dark Mode Depth & Atmosphere

Status: done

## Goal

The dark mode background feels flat and lifeless. Add subtle atmospheric depth without making it feel like a landing page — this is a utility tool, so the treatment should be felt more than seen.

## Plan

All changes in `globals.css` only — no new files, no JS, pure CSS.

### Phase 1: Atmosphere (adds texture and warmth)
- Bump `--card-bg` from `#1e1c19` to `#232019` so cards lift off the background
- Slight bump to `--border` in dark mode for consistency
- Add radial vignette gradient on body in dark mode (warm-tinted center, CSS-only)
- Add CSS-only SVG noise texture overlay at 2.5% opacity with `prefers-reduced-motion` support

### Phase 2: Structure (widens the surface stack for real depth)
- Bump `--card-bg` again from `#232019` to `#2c2720` — cards now have ~24 units of lift from page bg instead of ~9
- Strengthen `--border` from `#3a352f` to `#4a443d` — borders visible against card surface
- Fix `--accent-subtle` from `#2a1f1b` to `#3a2820` — warm tint reads as a distinct highlighted surface
- Increase dark shadow opacity ~50-65% across all sizes (sm: 0.3, md: 0.4, lg: 0.5) — shadows need more intensity to register on dark backgrounds
- Dial back body gradient center from `#1e1a16` to `#1c1915` — surfaces carry the depth now, gradient is just atmospheric warmth

## Log

- 2026-04-18 — Created plan
- 2026-04-18 — Phase 1: Implemented atmosphere changes (card contrast bump, radial vignette, noise texture overlay)
- 2026-04-18 — Verified build passes. Marked done.
- 2026-04-18 — Reopened. Phase 1 didn't go far enough — dark mode still flat, surfaces too compressed
- 2026-04-18 — Phase 2: Widened surface stack, strengthened borders/shadows, fixed accent-subtle. All changes in globals.css only
