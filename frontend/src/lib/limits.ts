// Mirrors the server-side caps in backend/internal/limits/limits.go.
// Only limits the UI actually surfaces are mirrored here — others stay server-only.
// When a server limit changes, both files change in the same commit.
export const MAX_CALENDAR_NAME_LENGTH = 100;
export const MAX_PARTICIPANT_NAME_LENGTH = 50;
export const MAX_USER_NAME_LENGTH = 50;
export const MAX_PARTICIPANTS_PER_CALENDAR = 100;
// Per-date availability note ceiling, counted in Unicode code points. Matches
// MaxNoteLength in the backend limits package and the DB check constraint.
// Use codePointCount() / Array.from(str).length when enforcing this —
// String.prototype.length counts UTF-16 code units and overcounts emoji.
export const MAX_NOTE_LENGTH = 140;

// Count a string's length in Unicode code points. Use this for note-length
// enforcement so the client counter stays in lockstep with the server's
// char_length() / utf8.RuneCountInString checks.
export function codePointCount(s: string): number {
  let count = 0;
  for (const _ of s) count += 1;
  return count;
}
