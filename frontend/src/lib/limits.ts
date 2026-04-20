// Mirrors the server-side caps in backend/internal/limits/limits.go.
// Only limits the UI actually surfaces are mirrored here — others stay server-only.
// When a server limit changes, both files change in the same commit.
export const MAX_CALENDAR_NAME_LENGTH = 100;
export const MAX_PARTICIPANT_NAME_LENGTH = 50;
export const MAX_PARTICIPANTS_PER_CALENDAR = 100;
