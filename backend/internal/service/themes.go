package service

import "math/rand/v2"

// ValidThemes is the allow-list of theme slugs a calendar may carry. The
// frontend holds the same set in src/lib/themes.ts; a drift test locks the Go
// list against this hard-coded value.
var ValidThemes = []string{
	"peach",
	"ocean",
	"forest",
	"plum",
	"lemon",
	"graphite",
}

// IsValidTheme reports whether slug is in the curated allow-list.
func IsValidTheme(slug string) bool {
	for _, t := range ValidThemes {
		if t == slug {
			return true
		}
	}
	return false
}

func pickRandomTheme() string {
	return ValidThemes[rand.IntN(len(ValidThemes))]
}
