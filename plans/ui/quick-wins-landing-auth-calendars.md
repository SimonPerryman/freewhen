# Quick wins: landing, sign-in, My calendars

Status: done

## Goal

Refactoring UI pass (PR A, part): calmer sign-in, no dead ends after it, and a useful My calendars page with a clear way to create one.

## Plan

- Landing: input and button stack full-width below 600px; hint "You'll sign in with your email to save it." shown only when signed out.
- Login: ink heading, left-aligned content, shared `TextField` + `Button`, validate on submit (no `required`/disabled-until-typed), logo in the header bar on `/login*`.
- Check email: address on its own line (`overflow-wrap: anywhere`), no "Sign in" link, "Resend link" with 30s cooldown, "Use a different email". The login page now passes `return_url` through to check-email so resend and "different email" keep it.
- My calendars: "+ New calendar" link-button by the H1 (36px visible, 44px tap area); designed empty state (`EmptyCalendars`: mini grid, pitch line, inline name field + Create).

## Log

- 2026-11-23 10:30 — Created plan
- 2026-11-23 11:10 — Implemented landing, login, check-email, header, My calendars + empty state; vitest added for each
- 2026-11-24 19:40 — Audit: shipped in #99 (stacked landing form with signed-out hint, calmer login, My calendars). Marked done.
