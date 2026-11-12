package idprovider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func withGoogle(t *testing.T, handler http.HandlerFunc) *Google {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewGoogle("client", "secret", "https://app.test/callback",
		WithGoogleUserInfoURL(srv.URL),
	)
}

func TestGoogleIdentify_SuccessReturnsIdentity(t *testing.T) {
	var gotAuth string
	g := withGoogle(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"sub":"google-123","email":"a@example.com","email_verified":true,"name":"Alice"}`)
	})

	id, err := g.Identify(context.Background(), &oauth2.Token{AccessToken: "tok"})
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if id.Sub != "google-123" || id.Email != "a@example.com" || id.Name != "Alice" || !id.EmailVerified {
		t.Errorf("identity = %+v", id)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("Authorization = %q, want Bearer ...", gotAuth)
	}
}

func TestGoogleIdentify_EmailVerifiedDefaultsFalse(t *testing.T) {
	for name, body := range map[string]string{
		"explicit false": `{"sub":"s","email":"a@example.com","email_verified":false}`,
		"missing":        `{"sub":"s","email":"a@example.com"}`,
	} {
		t.Run(name, func(t *testing.T) {
			g := withGoogle(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, body)
			})
			id, err := g.Identify(context.Background(), &oauth2.Token{AccessToken: "tok"})
			if err != nil {
				t.Fatalf("Identify: %v", err)
			}
			if id.EmailVerified {
				t.Errorf("EmailVerified = true, want false")
			}
		})
	}
}

func TestGoogleIdentify_Non2xxReturnsError(t *testing.T) {
	g := withGoogle(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "forbidden")
	})

	if _, err := g.Identify(context.Background(), &oauth2.Token{AccessToken: "tok"}); err == nil {
		t.Fatal("expected error on non-2xx userinfo response")
	}
}

func TestGoogleIdentify_MalformedJSONReturnsError(t *testing.T) {
	g := withGoogle(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{not json`)
	})

	if _, err := g.Identify(context.Background(), &oauth2.Token{AccessToken: "tok"}); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestGoogleIdentify_NotConfiguredReturnsError(t *testing.T) {
	g := NewGoogle("", "", "")
	if g.OAuthConfig() != nil {
		t.Fatal("OAuthConfig should be nil without credentials")
	}
	if _, err := g.Identify(context.Background(), &oauth2.Token{AccessToken: "tok"}); err == nil {
		t.Fatal("expected error when provider unconfigured")
	}
}

func TestRegistry_LookupAndIterationOrder(t *testing.T) {
	g1 := NewGoogle("a", "s", "https://x/cb")
	r := NewRegistry(nil, g1, nil)
	got, ok := r.Get("google")
	if !ok || got != g1 {
		t.Fatalf("Get(google) = %v, %v", got, ok)
	}
	if _, ok := r.Get("missing"); ok {
		t.Errorf("Get(missing) ok=true")
	}
	all := r.All()
	if len(all) != 1 || all[0].Name() != "google" {
		t.Errorf("All() = %+v", all)
	}
}

func TestRegistry_NilSafe(t *testing.T) {
	var r *Registry
	if _, ok := r.Get("google"); ok {
		t.Errorf("nil registry Get ok=true")
	}
	if all := r.All(); all != nil {
		t.Errorf("nil registry All = %v", all)
	}
}
