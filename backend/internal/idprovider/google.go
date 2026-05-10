package idprovider

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

// defaultGoogleUserInfoURL is Google's OIDC userinfo endpoint. Overridable
// per-instance via WithGoogleUserInfoURL for tests.
const defaultGoogleUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

// Google implements Provider against Google's OIDC userinfo endpoint.
type Google struct {
	oauth       *oauth2.Config
	userInfoURL string
}

// GoogleOption configures optional fields on the Google provider.
type GoogleOption func(*Google)

// WithGoogleOAuthEndpoint overrides the OAuth endpoint. Tests point this at an
// httptest.Server.
func WithGoogleOAuthEndpoint(ep oauth2.Endpoint) GoogleOption {
	return func(g *Google) {
		if g.oauth != nil {
			g.oauth.Endpoint = ep
		}
	}
}

// WithGoogleUserInfoURL overrides the userinfo URL. Tests point this at an
// httptest.Server.
func WithGoogleUserInfoURL(u string) GoogleOption {
	return func(g *Google) {
		if u != "" {
			g.userInfoURL = u
		}
	}
}

// NewGoogle constructs a Google identity provider. Returns a usable provider
// even when credentials are missing — OAuthConfig() will return nil and the
// route handler will gate on that. Mirrors calprovider.NewGoogle's behaviour.
func NewGoogle(clientID, clientSecret, redirectURL string, opts ...GoogleOption) *Google {
	var oauthCfg *oauth2.Config
	if clientID != "" && redirectURL != "" {
		oauthCfg = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     googleoauth.Endpoint,
		}
	}
	g := &Google{
		oauth:       oauthCfg,
		userInfoURL: defaultGoogleUserInfoURL,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Name implements Provider.
func (g *Google) Name() string { return "google" }

// OAuthConfig implements Provider.
func (g *Google) OAuthConfig() *oauth2.Config { return g.oauth }

// Identify implements Provider — calls Google's OIDC userinfo endpoint with
// the access token and returns the {sub, email, name} triple. The token is
// used once and not retained.
func (g *Google) Identify(ctx context.Context, token *oauth2.Token) (Identity, error) {
	if g.oauth == nil {
		return Identity{}, fmt.Errorf("google identity provider not configured")
	}
	client := g.oauth.Client(ctx, token)
	resp, err := client.Get(g.userInfoURL)
	if err != nil {
		return Identity{}, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Identity{}, fmt.Errorf("userinfo status %d", resp.StatusCode)
	}

	var payload struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Identity{}, fmt.Errorf("decode userinfo: %w", err)
	}
	return Identity{Sub: payload.Sub, Email: payload.Email, Name: payload.Name}, nil
}
