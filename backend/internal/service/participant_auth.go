package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

// ParticipantCaller identifies who is making a participant-scoped write.
// Identity comes only from credentials the caller holds — the session user
// and the raw participant token (X-Participant-Token) — never from a
// participant id in the request body.
type ParticipantCaller struct {
	UserID *string
	Token  string
}

// participantTokenBytes is the entropy of a participant token (256 bits).
const participantTokenBytes = 32

// newParticipantToken returns a fresh raw token and the SHA-256 hex digest
// that is persisted. Only the digest is stored, mirroring session tokens.
func newParticipantToken() (raw, hash string, err error) {
	b := make([]byte, participantTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate participant token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashParticipantToken(raw), nil
}

func hashParticipantToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// authorizeParticipantWrite is the single authorization decision for every
// mutation that targets a participant row. Rules:
//
//  1. ownerAllowed && caller is the calendar owner → allowed.
//  2. Target is linked to a user → caller's session user must match.
//  3. Target is anonymous with a stored token hash → caller's token must
//     hash to it (constant-time compare).
//  4. Target is anonymous with no hash → ErrForbidden. Tokens are issued only
//     at join (AddParticipant), so such a row can never be written.
//
// Returns the target participant, or store.ErrNotFound when the target does
// not exist on this calendar.
func (s *Service) authorizeParticipantWrite(ctx context.Context, cal model.Calendar, targetID string, caller ParticipantCaller, ownerAllowed bool) (model.Participant, error) {
	target, tokenHash, err := s.store.GetParticipantForAuth(ctx, targetID)
	if err != nil {
		return model.Participant{}, err
	}
	if target.CalendarID != cal.ID {
		return model.Participant{}, store.ErrNotFound
	}

	if ownerAllowed && caller.UserID != nil && *caller.UserID == cal.OwnerID {
		return target, nil
	}

	if target.UserID != nil {
		if caller.UserID != nil && *caller.UserID == *target.UserID {
			return target, nil
		}
		return model.Participant{}, ErrForbidden
	}

	if tokenHash == nil || caller.Token == "" {
		return model.Participant{}, ErrForbidden
	}
	got := hashParticipantToken(caller.Token)
	if subtle.ConstantTimeCompare([]byte(got), []byte(*tokenHash)) != 1 {
		return model.Participant{}, ErrForbidden
	}
	return target, nil
}
