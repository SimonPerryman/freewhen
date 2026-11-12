# Vote toggle saves a stale vote set

Status: done

## Goal

`handleToggleVote` in `frontend/src/lib/useAvailabilitySave.ts` updated
`votesByDateRef` inside a `setVotesByDate` updater and then called
`flushSave()` immediately. React only runs updaters eagerly when the fiber has
no pending work; otherwise they run later during render (and twice in
StrictMode). The save loop reads `votesByDateRef` synchronously before its
first `await`, so the PUT can carry the previous vote set and the next
refresh/realtime update reverts the user's vote.

## Plan

1. Add failing tests in `useAvailabilitySave.test.ts`: toggle a vote and assert
   the request body's `preferred` is the new set; rapid double toggles in one
   batch end in the right final state; StrictMode doesn't change the result.
2. Fix `handleToggleVote`: compute the next set from `votesByDateRef.current`
   outside the updater, write ref + state with that value, then save.
3. Apply the same rule to the other updaters with side effects: vote/note
   pruning in `handleToggle`, `handleRangeToggle`, `handleClearAvailability`
   (ref writes inside updaters), and `handleSaveNote` (analytics inside the
   updater). Add a notes ref so note handlers compute synchronously too.
4. Keep `seed`/`rehydrate` writing the refs synchronously so a handler fired in
   the same batch sees the seeded values.
5. Gate: vitest, oxlint, oxfmt on changed files, `pnpm build`.

## Log

- 2026-11-12 10:00 — Created plan
- 2026-11-12 10:10 — Added 6 tests; 3 vote tests failed on main (single toggle sent `[F1]` instead of `[F1, F2]`; toggle batched with a refresh sent `[]`; rapid double toggle's first PUT stale)
- 2026-11-12 10:25 — Fixed: refs are now the handlers' source of truth, written synchronously via `commitSelected`/`commitNotes`/`commitVotes` (dropped the effect-based ref sync); `seed`/`rehydrate` write refs too; vote/note pruning shared in `pruneNotesAndVotes`; note analytics moved out of the updater; `handleRetry` reads the ref. Component-level toggles (WhosMissingPanel, PreferencePill, NotesPill analytics in `setOpen` updaters) left alone — UI only, no saves
- 2026-11-12 10:35 — vitest 411/411, oxlint clean, oxfmt clean on changed files, `pnpm build` ok
