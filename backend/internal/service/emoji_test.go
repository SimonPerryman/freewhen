package service

import (
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/model"
)

var emojiTestNow = time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

func seeded(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed))
}

// uses builds n calendars using emoji, each last active daysAgo days before
// emojiTestNow.
func uses(emoji string, n int, daysAgo int) []model.EmojiUse {
	out := make([]model.EmojiUse, n)
	for i := range out {
		out[i] = model.EmojiUse{Emoji: emoji, LastActive: emojiTestNow.AddDate(0, 0, -daysAgo)}
	}
	return out
}

// allUsedOnceExcept returns one recent calendar for every default emoji
// other than skip.
func allUsedOnceExcept(skip ...string) []model.EmojiUse {
	var out []model.EmojiUse
	for _, e := range defaultEmojis {
		if slices.Contains(skip, e) {
			continue
		}
		out = append(out, uses(e, 1, 0)...)
	}
	return out
}

func TestPickEmoji(t *testing.T) {
	e := defaultEmojis
	allButE1 := slices.DeleteFunc(slices.Clone(e), func(s string) bool { return s == e[1] })

	tests := []struct {
		name  string
		usage []model.EmojiUse
		// want is the set of acceptable picks.
		want []string
	}{
		{
			name:  "no calendars picks any default",
			usage: nil,
			want:  e,
		},
		{
			name:  "single unused emoji is picked",
			usage: allUsedOnceExcept(e[4]),
			want:  []string{e[4]},
		},
		{
			name:  "unused emojis beat a long-idle one",
			usage: append(allUsedOnceExcept(e[2], e[7]), uses(e[2], 1, 3650)...),
			want:  []string{e[7]},
		},
		{
			name:  "all used: least-used wins",
			usage: append(append(allUsedOnceExcept(), uses(e[0], 1, 0)...), allUsedOnceExcept(e[9])...),
			want:  []string{e[9]},
		},
		{
			name: "all used: old heavy emoji loses to recent light one",
			// e[3] has three calendars idle for a year (3 × ~0.06 ≈ 0.18);
			// every other emoji has one calendar active today (1.0).
			usage: append(allUsedOnceExcept(e[3]), uses(e[3], 3, 365)...),
			want:  []string{e[3]},
		},
		{
			name: "half-life: one calendar idle 90 days weighs half",
			// e[5]: one at 90 days = 0.5. e[6]: one at 0 days = 1.0 plus the
			// baseline. Everything else: two at 0 days = 2.0.
			usage: append(append(allUsedOnceExcept(e[5]), allUsedOnceExcept(e[5], e[6])...),
				uses(e[5], 1, 90)...),
			want: []string{e[5]},
		},
		{
			name:  "all tied: any default",
			usage: allUsedOnceExcept(),
			want:  e,
		},
		{
			name:  "emojis outside the default set are ignored",
			usage: append(allUsedOnceExcept(e[11]), uses("🐙", 5, 0)...),
			want:  []string{e[11]},
		},
		{
			// e[1] weighs 2 (a future timestamp clamps to 1, plus 1); the
			// rest weigh 1, so anything but e[1] is acceptable.
			name: "future LastActive is clamped to weight 1",
			usage: append(allUsedOnceExcept(),
				model.EmojiUse{Emoji: e[1], LastActive: emojiTestNow.Add(48 * time.Hour)}),
			want: allButE1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for seed := range uint64(20) {
				got := pickEmoji(tt.usage, emojiTestNow, seeded(seed))
				if !slices.Contains(tt.want, got) {
					t.Fatalf("seed %d: pickEmoji = %q, want one of %q", seed, got, tt.want)
				}
			}
		})
	}
}

func TestPickEmoji_TiesAreSeededAndSpread(t *testing.T) {
	usage := allUsedOnceExcept(defaultEmojis[0], defaultEmojis[1], defaultEmojis[2])

	a := pickEmoji(usage, emojiTestNow, seeded(42))
	b := pickEmoji(usage, emojiTestNow, seeded(42))
	if a != b {
		t.Fatalf("same seed gave %q then %q", a, b)
	}

	seen := map[string]bool{}
	for seed := range uint64(200) {
		seen[pickEmoji(usage, emojiTestNow, seeded(seed))] = true
	}
	if len(seen) != 3 {
		t.Fatalf("tie-break over 200 seeds hit %d of 3 candidates: %v", len(seen), seen)
	}
}

func TestPickEmoji_FillsAllDefaultsBeforeRepeating(t *testing.T) {
	r := seeded(7)
	var usage []model.EmojiUse
	seen := map[string]bool{}
	for i := range defaultEmojis {
		got := pickEmoji(usage, emojiTestNow, r)
		if seen[got] {
			t.Fatalf("pick %d repeated %q before all defaults were used", i, got)
		}
		seen[got] = true
		usage = append(usage, model.EmojiUse{Emoji: got, LastActive: emojiTestNow})
	}
}

func TestPickEmoji_NilRandUsesGlobalSource(t *testing.T) {
	got := pickEmoji(nil, emojiTestNow, nil)
	if !slices.Contains(defaultEmojis, got) {
		t.Fatalf("pickEmoji = %q, not in curated set", got)
	}
}
