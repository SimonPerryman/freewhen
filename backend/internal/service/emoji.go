package service

import "math/rand/v2"

// defaultEmojis is the curated set new calendars draw from at create time.
// Avoid 📅 / 🗓️ (reads as inference failure) and avoid tonally loaded
// or topically narrow icons. Bias toward friendly, ambiguous shapes that
// look like personality rather than a topic guess.
var defaultEmojis = []string{
	"\u2728",     // ✨ sparkles
	"\U0001FAB4", // 🪴 potted plant
	"\U0001F33B", // 🌻 sunflower
	"\U0001F33F", // 🌿 herb
	"\U0001F38F", // 🎏 carp streamer
	"\U0001F388", // 🎈 balloon
	"\U0001F34B", // 🍋 lemon
	"\U0001F352", // 🍒 cherries
	"\U0001F30A", // 🌊 water wave
	"\U0001F98A", // 🦊 fox
	"\U0001F6FC", // 🛼 roller skate
	"\U0001FAA9", // 🪩 mirror ball
}

func pickRandomEmoji() string {
	return defaultEmojis[rand.IntN(len(defaultEmojis))]
}
