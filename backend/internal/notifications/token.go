// Package notifications wires the transactional email layer (finalised /
// nudge / unsubscribe) on top of the email sender. Tokens here are HMAC-signed
// participant ids used to authenticate the one-click List-Unsubscribe URL —
// no session, no DB lookup at issue time, replay-safe via expiry.
package notifications

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

// ErrInvalidToken is returned when the supplied unsubscribe token is
// malformed, expired, or fails signature verification.
var ErrInvalidToken = errors.New("invalid unsubscribe token")

// TokenSigner mints and verifies one-click unsubscribe tokens. The secret is
// read once at startup from NOTIFICATIONS_HMAC_SECRET (config layer) and never
// rotated mid-process — token rotation is a separate, deferred concern.
type TokenSigner struct {
	secret []byte
	ttl    time.Duration
}

// NewTokenSigner returns a signer using the given secret. ttl bounds how long
// a generated token stays valid after issue. Returns an error if the secret
// is shorter than 32 bytes.
func NewTokenSigner(secret string, ttl time.Duration) (*TokenSigner, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("notifications hmac secret must be at least 32 bytes")
	}
	if ttl == 0 {
		ttl = 365 * 24 * time.Hour
	}
	return &TokenSigner{secret: []byte(secret), ttl: ttl}, nil
}

// Sign produces a token of the form base64(participantID):exp:base64(mac).
// Unsubscribe links are typed by humans only via copy/paste from email
// clients, so the format prioritises straightforward parsing over compactness.
func (s *TokenSigner) Sign(participantID string) string {
	exp := time.Now().Add(s.ttl).Unix()
	body := encodeBody(participantID, exp)
	mac := s.mac(body)
	return body + "." + base64.RawURLEncoding.EncodeToString(mac)
}

// Verify validates the token's signature and expiry, returning the embedded
// participant id. ErrInvalidToken is returned for any failure mode — callers
// should not distinguish between expired and forged tokens.
func (s *TokenSigner) Verify(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", ErrInvalidToken
	}
	body, sig := parts[0], parts[1]
	expectedMac, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return "", ErrInvalidToken
	}
	if !hmac.Equal(s.mac(body), expectedMac) {
		return "", ErrInvalidToken
	}

	bodyParts := strings.Split(body, ":")
	if len(bodyParts) != 2 {
		return "", ErrInvalidToken
	}
	idBytes, err := base64.RawURLEncoding.DecodeString(bodyParts[0])
	if err != nil {
		return "", ErrInvalidToken
	}
	exp, err := strconv.ParseInt(bodyParts[1], 10, 64)
	if err != nil {
		return "", ErrInvalidToken
	}
	if time.Now().Unix() > exp {
		return "", ErrInvalidToken
	}
	return string(idBytes), nil
}

func (s *TokenSigner) mac(body string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(body))
	return mac.Sum(nil)
}

func encodeBody(participantID string, exp int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(participantID)) + ":" + strconv.FormatInt(exp, 10)
}
