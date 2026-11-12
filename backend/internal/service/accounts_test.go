package service

import (
	"context"
	"errors"
	"testing"

	"github.com/freewhen/backend/internal/idprovider"
	"github.com/freewhen/backend/internal/model"
)

// fakeAccountStore is an in-memory AccountStore. raceOnCreate simulates a
// concurrent sign-in that inserts the same email between our lookup and our
// insert.
type fakeAccountStore struct {
	byEmail      map[string]model.User
	byProvider   map[string]model.User
	creates      int
	raceOnCreate *model.User
	findErr      error
}

func newFakeAccountStore() *fakeAccountStore {
	return &fakeAccountStore{byEmail: map[string]model.User{}, byProvider: map[string]model.User{}}
}

func (f *fakeAccountStore) FindUserByProviderID(_ context.Context, provider, sub string) (model.User, error) {
	if u, ok := f.byProvider[provider+":"+sub]; ok {
		return u, nil
	}
	return model.User{}, model.ErrNotFound
}

func (f *fakeAccountStore) FindUserByEmail(_ context.Context, email string) (model.User, error) {
	if f.findErr != nil {
		return model.User{}, f.findErr
	}
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return model.User{}, model.ErrNotFound
}

func (f *fakeAccountStore) CreateUser(_ context.Context, email, name, provider, sub string) (model.User, error) {
	f.creates++
	if f.raceOnCreate != nil {
		f.byEmail[email] = *f.raceOnCreate
		return model.User{}, model.ErrUserExists
	}
	if _, ok := f.byEmail[email]; ok {
		return model.User{}, model.ErrUserExists
	}
	u := model.User{ID: "u_" + email, Email: email, Name: name, Provider: provider, ProviderID: sub}
	f.byEmail[email] = u
	if sub != "" {
		f.byProvider[provider+":"+sub] = u
	}
	return u, nil
}

func TestFindOrCreateOAuthUser_ExistingProviderLinkWinsEvenIfUnverified(t *testing.T) {
	st := newFakeAccountStore()
	existing := model.User{ID: "u1", Email: "a@example.com"}
	st.byProvider["google:sub1"] = existing
	u, err := NewAccounts(st).FindOrCreateOAuthUser(context.Background(), "google",
		idprovider.Identity{Sub: "sub1", Email: "a@example.com", EmailVerified: false})
	if err != nil || u.ID != "u1" {
		t.Fatalf("got %+v, %v; want existing u1", u, err)
	}
}

func TestFindOrCreateOAuthUser_UnverifiedEmailNeitherLinksNorCreates(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   idprovider.Identity
	}{
		{"unverified", idprovider.Identity{Sub: "s", Email: "a@example.com", EmailVerified: false}},
		{"empty email", idprovider.Identity{Sub: "s", Email: "", EmailVerified: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeAccountStore()
			st.byEmail["a@example.com"] = model.User{ID: "victim", Email: "a@example.com"}
			_, err := NewAccounts(st).FindOrCreateOAuthUser(context.Background(), "google", tc.id)
			if !errors.Is(err, ErrEmailNotVerified) {
				t.Fatalf("err = %v, want ErrEmailNotVerified", err)
			}
			if st.creates != 0 {
				t.Fatalf("CreateUser called %d times", st.creates)
			}
		})
	}
}

func TestFindOrCreateOAuthUser_VerifiedLinksByEmail(t *testing.T) {
	st := newFakeAccountStore()
	st.byEmail["a@example.com"] = model.User{ID: "magic", Email: "a@example.com"}
	u, err := NewAccounts(st).FindOrCreateOAuthUser(context.Background(), "google",
		idprovider.Identity{Sub: "s", Email: "a@example.com", EmailVerified: true})
	if err != nil || u.ID != "magic" || st.creates != 0 {
		t.Fatalf("got %+v, %v, creates=%d; want link to magic", u, err, st.creates)
	}
}

func TestFindOrCreateOAuthUser_VerifiedCreates(t *testing.T) {
	st := newFakeAccountStore()
	u, err := NewAccounts(st).FindOrCreateOAuthUser(context.Background(), "google",
		idprovider.Identity{Sub: "s", Email: "new@example.com", Name: "New", EmailVerified: true})
	if err != nil || u.Email != "new@example.com" || u.Provider != "google" || u.ProviderID != "s" || u.Name != "New" {
		t.Fatalf("got %+v, %v", u, err)
	}
}

func TestFindOrCreateByEmail_LostCreateRaceReturnsWinner(t *testing.T) {
	st := newFakeAccountStore()
	winner := model.User{ID: "winner", Email: "a@example.com"}
	st.raceOnCreate = &winner
	u, err := NewAccounts(st).FindOrCreateByEmail(context.Background(), "a@example.com", "", "email", "")
	if err != nil {
		t.Fatalf("lost race must not fail: %v", err)
	}
	if u.ID != "winner" {
		t.Fatalf("got %+v, want the concurrently created user", u)
	}
}

func TestFindOrCreateByEmail_LookupErrorPropagates(t *testing.T) {
	st := newFakeAccountStore()
	boom := errors.New("db down")
	st.findErr = boom
	if _, err := NewAccounts(st).FindOrCreateByEmail(context.Background(), "a@example.com", "", "email", ""); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want db error", err)
	}
	if st.creates != 0 {
		t.Fatal("must not create after a failed lookup")
	}
}
