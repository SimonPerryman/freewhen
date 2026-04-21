// Package passphrase generates share-friendly passphrases used to gate access
// to a calendar. The format is four words from a curated short wordlist,
// joined by hyphens: word-word-word-word (e.g. "frost-tulip-raven-stamp").
//
// See plans/password-protected-calendars.md for the rationale (entropy,
// plaintext storage tradeoff).
package passphrase

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// WordCount is the number of words in a generated passphrase. Four short
// words from the wordlist is ~41 bits of entropy — more than enough against
// online guessing with the per-calendar + per-IP rate limit in place.
const WordCount = 4

// Generate returns a new passphrase: four randomly-chosen words from the
// wordlist, joined by "-". Panics only if crypto/rand fails, which the Go
// runtime treats as unrecoverable anyway.
func Generate() string {
	parts := make([]string, WordCount)
	n := big.NewInt(int64(len(words)))
	for i := 0; i < WordCount; i++ {
		idx, err := rand.Int(rand.Reader, n)
		if err != nil {
			panic("passphrase: crypto/rand failed: " + err.Error())
		}
		parts[i] = words[idx.Int64()]
	}
	return strings.Join(parts, "-")
}
