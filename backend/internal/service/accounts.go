package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/freewhen/backend/internal/idprovider"
	"github.com/freewhen/backend/internal/model"
)

// ErrEmailNotVerified is returned by FindOrCreateOAuthUser when the identity
// has no existing provider link and its email isn't verified by the provider.
var ErrEmailNotVerified = errors.New("identity email not verified")

// AccountStore is the persistence the account flows need.
//
// CreateUser must return model.ErrUserExists (not a generic error) when the
// email is already taken, so a sign-in that loses a creation race can fall
// back to the row the winner inserted.
type AccountStore interface {
	FindUserByProviderID(ctx context.Context, provider, providerID string) (model.User, error)
	FindUserByEmail(ctx context.Context, email string) (model.User, error)
	CreateUser(ctx context.Context, email, name, provider, providerID string) (model.User, error)
}

// Accounts owns sign-in account resolution: find an existing user, link by
// email, or create one. Kept separate from Service because it has no
// calendar dependencies.
type Accounts struct {
	store AccountStore
}

func NewAccounts(st AccountStore) *Accounts {
	return &Accounts{store: st}
}

// FindOrCreateOAuthUser resolves an identity-provider sign-in to a user.
//
//  1. An existing (provider, sub) link always wins.
//  2. Otherwise the email decides — link to an existing account with that
//     email (e.g. one created by magic link) or create a new one. Both key
//     on the address, so both require the provider to vouch for it
//     (EmailVerified && Email != ""); else ErrEmailNotVerified. Without this
//     an attacker-controlled provider account carrying someone else's
//     unverified email could take over their FreeWhen account.
//
// identity.Email must already be normalised by the caller.
func (a *Accounts) FindOrCreateOAuthUser(ctx context.Context, providerName string, identity idprovider.Identity) (model.User, error) {
	user, err := a.store.FindUserByProviderID(ctx, providerName, identity.Sub)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		return model.User{}, fmt.Errorf("find user by provider ID: %w", err)
	}

	if !identity.EmailVerified || identity.Email == "" {
		return model.User{}, ErrEmailNotVerified
	}
	return a.FindOrCreateByEmail(ctx, identity.Email, identity.Name, providerName, identity.Sub)
}

// FindOrCreateByEmail returns the user with this (normalised) email, creating
// one with the given profile/provider fields if none exists. Race-safe: when
// two sign-ins for the same new address run concurrently, the loser's insert
// conflicts on users.email and it returns the winner's row instead of failing.
func (a *Accounts) FindOrCreateByEmail(ctx context.Context, email, name, provider, providerID string) (model.User, error) {
	user, err := a.store.FindUserByEmail(ctx, email)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		return model.User{}, fmt.Errorf("find user by email: %w", err)
	}

	user, err = a.store.CreateUser(ctx, email, name, provider, providerID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, model.ErrUserExists) {
		return model.User{}, err
	}
	// Lost the race: another request created the row between our lookup
	// and insert. It is now committed, so the re-read must find it.
	user, err = a.store.FindUserByEmail(ctx, email)
	if err != nil {
		return model.User{}, fmt.Errorf("find user after create conflict: %w", err)
	}
	return user, nil
}
