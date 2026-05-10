// Package idprovider defines the identity-provider seam: the interface the
// sign-in handler talks to instead of any one OAuth provider's API directly.
// Adding a new provider (Microsoft, Apple, GitHub) means adding a file to this
// package and registering it — no handler or route changes required.
//
// Sibling to calprovider, kept separate on purpose: identity tokens mint a
// persisted session and use OIDC scopes; calendar tokens are single-use and
// use provider-specific scopes. Entangling them through one interface would
// bake the wrong lifecycle into both.
package idprovider

import (
	"context"

	"golang.org/x/oauth2"
)

// Provider is the contract every identity provider implements. One real op
// (resolve identity from a freshly-issued token), plus the OAuth surface the
// handler uses to drive the round-trip. The handler runs Exchange — providers
// never persist tokens.
type Provider interface {
	// Name is the URL-safe identifier ("google", "microsoft", "apple"). Used
	// in route path params, cookie paths, and as the stored provider key on
	// the users row.
	Name() string

	// OAuthConfig returns the OAuth client used for sign-in. Returns nil when
	// credentials aren't configured for this environment — the route handler
	// gates on this and returns 503.
	OAuthConfig() *oauth2.Config

	// Identify resolves the freshly-issued token into a stable user identity.
	// Implementations may call a userinfo endpoint or decode an ID token,
	// whichever the provider supports — taking *oauth2.Token (not just the
	// access string) lets each provider pick its source. The handler never
	// sees that choice. Returned Email may be "" if the provider withholds
	// it; the handler does any canonicalisation.
	Identify(ctx context.Context, token *oauth2.Token) (Identity, error)
}

// Identity is the provider-agnostic shape the sign-in handler consumes. Sub is
// opaque per-provider; the (provider name, sub) pair keys the users row.
type Identity struct {
	Sub   string
	Email string
	Name  string
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
