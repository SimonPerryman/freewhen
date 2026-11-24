// Emojis an owner can choose for a calendar in Settings. Mirrors
// pickableEmojis in backend/internal/service/emoji.go (the server rejects
// anything else). The first 12 are the auto-pick set. A chosen emoji may
// repeat one the owner's other calendars use.
export const PICKABLE_EMOJIS = [
  "✨",
  "🪴",
  "🌻",
  "🌿",
  "🎏",
  "🎈",
  "🍋",
  "🍒",
  "🌊",
  "🦊",
  "🛼",
  "🪩",
  "🎉",
  "🎂",
  "🎄",
  "🍻",
  "☕",
  "🍕",
  "🎲",
  "🎸",
  "📚",
  "🎾",
  "🚲",
  "🧗",
] as const;
