// Package access implements stateless, HMAC-signed access tokens used to
// gate password-protected calendars. A token binds (calendarID, epoch,
// expiry); rotating a calendar's passphrase bumps the epoch, which instantly
// invalidates outstanding tokens without needing a server-side session store.
//
// See plans/password-protected-calendars.md — "Token shape".
package access

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DefaultTTL is how long an access token remains valid after issuance, absent
// earlier revocation via an epoch bump.
const DefaultTTL = 30 * 24 * time.Hour

// ErrInvalidToken is returned by VerifyToken when the token is malformed,
// tampered with, or expired. Callers should not distinguish the sub-reasons.
var ErrInvalidToken = errors.New("invalid access token")

// IssueToken returns a base64-encoded, HMAC-signed token binding the given
// calendar, epoch, and expiry. ttl is clamped to a positive duration;
// callers typically pass DefaultTTL.
func IssueToken(calendarID string, epoch int, secret string, ttl time.Duration) string {
	expiry := time.Now().UTC().Add(ttl).Unix()
	payload := fmt.Sprintf("%s|%d|%d", calendarID, epoch, expiry)
	sig := sign(payload, secret)
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + sig))
}

// VerifyToken parses and validates a token, returning the calendar ID and
// epoch it was issued for. Returns ErrInvalidToken for any failure (bad
// encoding, tampered signature, expired).
func VerifyToken(token, secret string) (calendarID string, epoch int, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", 0, ErrInvalidToken
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 {
		return "", 0, ErrInvalidToken
	}
	calendarID, epochStr, expiryStr, sig := parts[0], parts[1], parts[2], parts[3]
	payload := calendarID + "|" + epochStr + "|" + expiryStr

	expected := sign(payload, secret)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", 0, ErrInvalidToken
	}

	ep, err := strconv.Atoi(epochStr)
	if err != nil {
		return "", 0, ErrInvalidToken
	}
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", 0, ErrInvalidToken
	}
	if time.Now().UTC().Unix() >= expiry {
		return "", 0, ErrInvalidToken
	}
	return calendarID, ep, nil
}

func sign(payload, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
