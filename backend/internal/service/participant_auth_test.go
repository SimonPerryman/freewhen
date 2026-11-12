package service

import (
	"context"
	"errors"
	"testing"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

// authFixture: calendar "cal1" owned by "owner1"; participants
//   - "anon"   anonymous, token "tok-anon"
//   - "legacy" anonymous, no token hash (pre-migration row)
//   - "linked" linked to user "u1"
//   - "ownerp" linked to "owner1"
func authFixture() *mockStore {
	u1 := "u1"
	owner := "owner1"
	parts := map[string]model.Participant{
		"anon":   {ID: "anon", CalendarID: "cal1"},
		"legacy": {ID: "legacy", CalendarID: "cal1"},
		"linked": {ID: "linked", CalendarID: "cal1", UserID: &u1},
		"ownerp": {ID: "ownerp", CalendarID: "cal1", UserID: &owner},
	}
	return &mockStore{
		getParticipant: func(_ context.Context, id string) (model.Participant, error) {
			p, ok := parts[id]
			if !ok {
				return model.Participant{}, store.ErrNotFound
			}
			return p, nil
		},
		tokenHashes: map[string]string{"anon": hashParticipantToken("tok-anon")},
		setAvailability: func(_ context.Context, _ string, _ []string, _ map[string]string, _ map[string]bool) error {
			return nil
		},
		mergeAvailabilityDates: func(_ string, _ []string) error { return nil },
		updateParticipantName: func(_ context.Context, calendarID, pid, name string) (model.Participant, error) {
			return model.Participant{ID: pid, CalendarID: calendarID, Name: name}, nil
		},
		updateParticipantEmail: func(calendarID, pid string, email *string) (model.Participant, error) {
			return model.Participant{ID: pid, CalendarID: calendarID, Email: email}, nil
		},
		claimParticipant: func(calendarID, pid, userID string) (model.Participant, error) {
			return model.Participant{ID: pid, CalendarID: calendarID, UserID: &userID}, nil
		},
	}
}

func strp(s string) *string { return &s }

func TestNewParticipantToken_HashMatchesAndIsRandom(t *testing.T) {
	raw1, hash1, err := newParticipantToken()
	if err != nil {
		t.Fatal(err)
	}
	raw2, _, _ := newParticipantToken()
	if raw1 == raw2 {
		t.Fatal("tokens must differ")
	}
	if len(raw1) < 40 {
		t.Fatalf("token too short: %d", len(raw1))
	}
	if hash1 != hashParticipantToken(raw1) || hash1 == raw1 {
		t.Fatal("hash must be the SHA-256 digest of the raw token")
	}
}

func TestSetAvailability_Authorization(t *testing.T) {
	cases := []struct {
		name   string
		target string
		caller ParticipantCaller
		want   error
	}{
		{"anon with own token", "anon", ParticipantCaller{Token: "tok-anon"}, nil},
		{"anon without token", "anon", ParticipantCaller{}, ErrForbidden},
		{"anon with wrong token", "anon", ParticipantCaller{Token: "tok-other"}, ErrForbidden},
		{"anon target, signed-in stranger", "anon", ParticipantCaller{UserID: strp("u2")}, ErrForbidden},
		{"linked self", "linked", ParticipantCaller{UserID: strp("u1")}, nil},
		{"linked, other user", "linked", ParticipantCaller{UserID: strp("u2")}, ErrForbidden},
		{"linked, anonymous caller", "linked", ParticipantCaller{Token: "tok-anon"}, ErrForbidden},
		{"owner cannot write another's availability", "linked", ParticipantCaller{UserID: strp("owner1")}, ErrForbidden},
		{"owner's own row by anon caller", "ownerp", ParticipantCaller{}, ErrForbidden},
		{"unknown participant", "nope", ParticipantCaller{}, store.ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New(authFixture(), nil, AccessConfig{Secret: testSecret})
			_, err := s.SetAvailability(context.Background(), "cal1", tc.target, tc.caller, []string{"2026-05-01"}, nil, nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSetAvailability_ForbiddenDoesNotWrite(t *testing.T) {
	ms := authFixture()
	ms.setAvailability = func(_ context.Context, _ string, _ []string, _ map[string]string, _ map[string]bool) error {
		t.Fatal("store.SetAvailability must not be called for a forbidden write")
		return nil
	}
	s := New(ms, nil, AccessConfig{Secret: testSecret})
	if _, err := s.SetAvailability(context.Background(), "cal1", "ownerp", ParticipantCaller{}, nil, nil, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestSetAvailability_LegacyTrustOnFirstUse(t *testing.T) {
	ms := authFixture()
	s := New(ms, nil, AccessConfig{Secret: testSecret})

	minted, err := s.SetAvailability(context.Background(), "cal1", "legacy", ParticipantCaller{}, []string{"2026-05-01"}, nil, nil)
	if err != nil {
		t.Fatalf("first legacy write: %v", err)
	}
	if minted == "" {
		t.Fatal("expected a minted token on first legacy write")
	}
	if ms.tokenHashes["legacy"] != hashParticipantToken(minted) {
		t.Fatal("stored hash must match the minted token")
	}

	// Second caller without the token is now rejected.
	if _, err := s.SetAvailability(context.Background(), "cal1", "legacy", ParticipantCaller{}, nil, nil, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("second tokenless write: err = %v, want ErrForbidden", err)
	}
	// The holder of the minted token is accepted and nothing new is minted.
	again, err := s.SetAvailability(context.Background(), "cal1", "legacy", ParticipantCaller{Token: minted}, nil, nil, nil)
	if err != nil || again != "" {
		t.Fatalf("token holder: minted=%q err=%v", again, err)
	}
}

func TestSetAvailability_LegacyRaceLoserForbidden(t *testing.T) {
	ms := authFixture()
	ms.tokenRaceLost = true
	s := New(ms, nil, AccessConfig{Secret: testSecret})
	if _, err := s.SetAvailability(context.Background(), "cal1", "legacy", ParticipantCaller{}, nil, nil, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestImportAvailability_OtherParticipantForbidden(t *testing.T) {
	s := New(authFixture(), nil, AccessConfig{Secret: testSecret})
	// Signed-in u2 tries to import into u1's row.
	if _, err := s.ImportAvailability(context.Background(), "cal1", "linked", ParticipantCaller{UserID: strp("u2")}, []string{"2026-05-01"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if _, err := s.ImportAvailability(context.Background(), "cal1", "linked", ParticipantCaller{UserID: strp("u1")}, []string{"2026-05-01"}); err != nil {
		t.Fatalf("self import: %v", err)
	}
}

func TestUpdateParticipantEmail_Authorization(t *testing.T) {
	s := New(authFixture(), nil, AccessConfig{Secret: testSecret})
	email := strp("a@b.com")
	if _, _, err := s.UpdateParticipantEmail(context.Background(), "cal1", "anon", ParticipantCaller{}, email); !errors.Is(err, ErrForbidden) {
		t.Fatalf("no token: err = %v, want ErrForbidden", err)
	}
	if _, _, err := s.UpdateParticipantEmail(context.Background(), "cal1", "anon", ParticipantCaller{Token: "tok-anon"}, email); err != nil {
		t.Fatalf("own token: %v", err)
	}
	if _, _, err := s.UpdateParticipantEmail(context.Background(), "cal1", "anon", ParticipantCaller{UserID: strp("owner1")}, email); err != nil {
		t.Fatalf("owner: %v", err)
	}
}

func TestClaimParticipant_RequiresToken(t *testing.T) {
	s := New(authFixture(), nil, AccessConfig{Secret: testSecret})
	ctx := context.Background()

	if _, err := s.ClaimParticipant(ctx, "cal1", "anon", ParticipantCaller{UserID: strp("u2")}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("claim without token: err = %v, want ErrForbidden", err)
	}
	// Even the owner can't claim someone else's anonymous row.
	if _, err := s.ClaimParticipant(ctx, "cal1", "anon", ParticipantCaller{UserID: strp("owner1")}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("owner claim: err = %v, want ErrForbidden", err)
	}
	if _, err := s.ClaimParticipant(ctx, "cal1", "linked", ParticipantCaller{UserID: strp("u2"), Token: "x"}); !errors.Is(err, store.ErrAlreadyLinked) {
		t.Fatalf("claim linked: err = %v, want ErrAlreadyLinked", err)
	}
	p, err := s.ClaimParticipant(ctx, "cal1", "anon", ParticipantCaller{UserID: strp("u2"), Token: "tok-anon"})
	if err != nil {
		t.Fatalf("claim with token: %v", err)
	}
	if p.UserID == nil || *p.UserID != "u2" {
		t.Fatalf("claimed user = %v, want u2", p.UserID)
	}
}

func TestAddParticipant_IssuesTokenForAnonymousOnly(t *testing.T) {
	ms := authFixture()
	ms.addParticipant = func(_ context.Context, calendarID, name string, userID *string) (model.Participant, error) {
		return model.Participant{ID: "new", CalendarID: calendarID, Name: name, UserID: userID}, nil
	}
	s := New(ms, nil, AccessConfig{Secret: testSecret})

	_, tok, err := s.AddParticipant(context.Background(), "cal1", "Anon", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" || ms.addedTokenHash == nil || *ms.addedTokenHash != hashParticipantToken(tok) {
		t.Fatalf("anonymous join must store hash of returned token (tok=%q hash=%v)", tok, ms.addedTokenHash)
	}

	_, tok, err = s.AddParticipant(context.Background(), "cal1", "Signed", strp("u9"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "" || ms.addedTokenHash != nil {
		t.Fatalf("signed-in join must not issue a token (tok=%q hash=%v)", tok, ms.addedTokenHash)
	}
}
