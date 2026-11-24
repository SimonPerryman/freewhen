package service

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/freewhen/backend/internal/model"
)

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

// emojiHalfLife is how long it takes a calendar's claim on its emoji to halve.
// A calendar active today weighs 1, one idle for 90 days weighs 0.5, one idle
// for a year about 0.06.
const emojiHalfLife = 90 * 24 * time.Hour

// pickEmoji chooses an emoji for a new calendar from defaultEmojis, avoiding
// the owner's existing ones.
//
// Each existing calendar adds 0.5^(idle/emojiHalfLife) to its emoji's weight,
// where idle is now − LastActive (clamped at 0). The emoji with the lowest
// total weight wins and ties are broken uniformly at random. Unused emojis
// weigh 0, so they always go first; once all are taken, the least-used wins,
// with recent calendars counting for more than old or idle ones. Emojis
// outside defaultEmojis are ignored.
//
// r is the random source for tie-breaks; nil uses the global source.
func pickEmoji(usage []model.EmojiUse, now time.Time, r *rand.Rand) string {
	weights := make(map[string]float64, len(defaultEmojis))
	for _, e := range defaultEmojis {
		weights[e] = 0
	}
	for _, u := range usage {
		if _, ok := weights[u.Emoji]; !ok {
			continue
		}
		idle := max(now.Sub(u.LastActive), 0)
		weights[u.Emoji] += math.Pow(0.5, float64(idle)/float64(emojiHalfLife))
	}

	lowest := math.Inf(1)
	for _, w := range weights {
		lowest = min(lowest, w)
	}
	// Iterate defaultEmojis (not the map) so candidate order is stable and a
	// seeded r gives a deterministic pick. Compare exactly: any use, however
	// old, keeps a positive weight, so unused emojis always go first.
	var candidates []string
	for _, e := range defaultEmojis {
		if weights[e] == lowest {
			candidates = append(candidates, e)
		}
	}

	if r == nil {
		return candidates[rand.IntN(len(candidates))]
	}
	return candidates[r.IntN(len(candidates))]
}
