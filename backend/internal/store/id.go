package store

import "crypto/rand"

const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// CalendarIDLength is the length of newly generated calendar IDs. For a
// calendar without a passphrase the ID is the only secret, so it carries
// ~62 bits of entropy (36^12). Older calendars keep their 8-char IDs; nothing
// validates ID length, so both resolve. See
// plans/tech-debt/calendar-id-entropy-and-lookup-rate-limit.md.
const CalendarIDLength = 12

// maxUnbiasedByte is the largest multiple of len(alphabet) that fits in a
// byte (252 for 36 chars). Bytes at or above it are rejected so `b % 36` is
// uniform rather than favouring the first 256%36 characters.
const maxUnbiasedByte = 256 - (256 % len(alphabet))

// generateID returns n characters drawn uniformly from alphabet using
// crypto/rand with rejection sampling.
func generateID(n int) string {
	out := make([]byte, 0, n)
	// Over-draw a little: ~1.6% of bytes are rejected, so one read nearly
	// always suffices.
	buf := make([]byte, n+n/4+4)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		for _, b := range buf {
			if int(b) >= maxUnbiasedByte {
				continue
			}
			out = append(out, alphabet[int(b)%len(alphabet)])
			if len(out) == n {
				break
			}
		}
	}
	return string(out)
}
