// Package calprovider defines the calendar-provider seam: the interface the
// handler talks to instead of any one provider's API directly. Adding a new
// provider (Microsoft Graph, Apple/CalDAV) means adding a file to this package
// and registering it — no handler or route changes required.
package calprovider

import (
	"context"
	"time"

	"github.com/freewhen/backend/internal/availability"

	"golang.org/x/oauth2"
)

// Provider is the contract every calendar provider implements. Two real
// operations: free/busy import (requires OAuth) and an "add event" deep link
// (pure URL builder, no token).
type Provider interface {
	// Name is the URL-safe identifier ("google", "microsoft"). Used in route
	// path params, cookie paths, and analytics.
	Name() string

	// OAuthConfig returns the OAuth client used to obtain a one-shot access
	// token for FreeBusy. The handler runs the exchange — providers never
	// persist tokens.
	OAuthConfig() *oauth2.Config

	// FreeBusy reads busy intervals for the user's primary calendar in the
	// given window. tz is the IANA timezone the caller wants the provider to
	// interpret per-day boundaries in.
	FreeBusy(ctx context.Context, accessToken string, start, end time.Time, tz string) ([]availability.TimeRange, error)

	// AddEventURL returns a provider-specific URL that opens the user's
	// calendar UI with the event pre-filled. No API call, no token.
	AddEventURL(event ExportEvent) string
}

// ExportEvent is the provider-agnostic shape passed to AddEventURL. All-day
// is the only mode FreeWhen exports today.
type ExportEvent struct {
	Title       string
	Description string
	Date        time.Time
	AllDay      bool
}

// Registry holds the providers configured for this server. Lookup is by Name.
// Iteration order matches construction order so route registration is stable.
type Registry struct {
	byName map[string]Provider
	order  []string
}

// NewRegistry builds a registry from the given providers. nil entries are
// skipped — callers can pass providers that return nil from a constructor
// when their config is missing without filtering at the call site.
func NewRegistry(providers ...Provider) *Registry {
	r := &Registry{byName: map[string]Provider{}}
	for _, p := range providers {
		if p == nil {
			continue
		}
		name := p.Name()
		if _, exists := r.byName[name]; exists {
			continue
		}
		r.byName[name] = p
		r.order = append(r.order, name)
	}
	return r
}

// Get returns the provider with the given name, or (nil, false).
func (r *Registry) Get(name string) (Provider, bool) {
	if r == nil {
		return nil, false
	}
	p, ok := r.byName[name]
	return p, ok
}

// All returns the providers in registration order.
func (r *Registry) All() []Provider {
	if r == nil {
		return nil
	}
	out := make([]Provider, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.byName[name])
	}
	return out
}
