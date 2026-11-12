package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestIsRelativePath pins the open-redirect guard. The Google + magic-link
// flows both compose `frontendURL + returnURL`, so the invariant is "starts
// with a single forward slash, host can't be hijacked." Auth audit F10/F14:
// document with a test so a future refactor that breaks the invariant fails
// loudly.
func TestIsRelativePath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"root", "/", true},
		{"plain path", "/calendars/abc", true},
		{"path with query", "/calendars/abc?x=1", true},

		// Protocol-relative — would let the browser treat "evil.com" as the host.
		{"protocol relative", "//evil.com", false},
		{"protocol relative with path", "//evil.com/x", false},

		// Single leading slash + backslash. Browsers normalise `\` → `/`, so
		// if this value is ever followed without the frontendURL prefix
		// (e.g. by the frontend) `/\evil.com` becomes `//evil.com`. Reject
		// backslashes anywhere — no legitimate path needs one.
		{"slash backslash", "/\\evil.com", false},
		{"slash double backslash", "/\\\\evil.com", false},
		{"backslash mid path", "/path\\evil", false},

		// Tabs/newlines are stripped by the URL parser, so `/\t/evil.com`
		// collapses to `//evil.com`. Reject whitespace and control chars.
		{"slash tab slash", "/\t/evil.com", false},
		{"slash newline slash", "/\n/evil.com", false},
		{"slash carriage return", "/\r/evil.com", false},
		{"nul byte", "/\x00/evil.com", false},
		{"del byte", "/\x7f", false},
		{"embedded space", "/path with space", false},
		{"nbsp", "/ /evil.com", false},

		// Percent-encoded slashes — also just path content once the host is
		// fixed by the frontendURL prefix.
		{"slash percent encoded slashes", "/%2f%2fevil.com", true},

		// No leading slash at all — must be rejected.
		{"backslashes only", "\\\\evil.com", false},
		{"absolute http url", "http://evil.com", false},
		{"absolute https url", "https://evil.com/path", false},
		{"javascript scheme", "javascript:alert(1)", false},
		{"data scheme", "data:text/html,evil", false},
		{"bare host", "evil.com", false},

		// Whitespace prefix would not be normalised away — reject anything
		// that doesn't *literally* begin with "/".
		{"leading space slash", " /path", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRelativePath(tc.in); got != tc.want {
				t.Fatalf("isRelativePath(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestOAuthCallback_RedirectLocationStaysOnFrontend pins the integration-
// shape invariant: regardless of what the attacker stuffs into state.ReturnURL,
// the final Location header always starts with `frontendURL + "/"`. This is
// belt-and-braces over isRelativePath — if someone refactors OAuthCallback
// and accidentally drops the `isRelativePath` guard, this test catches it
// because the Location prefix would no longer hold for malicious inputs.
func TestOAuthCallback_RedirectLocationStaysOnFrontend(t *testing.T) {
	const frontend = "https://app.test"

	inputs := []string{
		"",
		"/",
		"/calendars/abc",
		"//evil.com",
		"//evil.com/x",
		"/\\evil.com",
		"/\\\\evil.com",
		"/%2f%2fevil.com",
		"\\\\evil.com",
		"http://evil.com",
		"https://evil.com/x",
		"javascript:alert(1)",
		"evil.com",
	}

	for _, ret := range inputs {
		t.Run(ret, func(t *testing.T) {
			fg := newFakeGoogle(t)
			st := newFakeAuthStore()
			h := newGoogleTestAuthHandler(t, st, fg)

			state, err := json.Marshal(oauthState{Nonce: "n", ReturnURL: ret})
			if err != nil {
				t.Fatal(err)
			}
			req := requestWithProvider(http.MethodGet,
				"/auth/google/callback?code=abc&state="+url.QueryEscape(string(state)), "google")
			req.AddCookie(&http.Cookie{Name: "oauth_nonce", Value: "n"})
			rec := httptest.NewRecorder()
			h.OAuthCallback(rec, req)

			if rec.Code != http.StatusTemporaryRedirect {
				t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
			}
			loc := rec.Header().Get("Location")
			if !strings.HasPrefix(loc, frontend+"/") {
				t.Fatalf("Location must stay on frontend, got %q for return_url=%q", loc, ret)
			}
			// And — crucially — must not have escaped to a different host via
			// the `//evil.com` trick. After `frontend + "/"`, the next char
			// must not be another `/`, which would make the browser parse
			// `evil.com` as the host.
			rest := strings.TrimPrefix(loc, frontend+"/")
			if strings.HasPrefix(rest, "/") {
				t.Fatalf("Location escapes host with protocol-relative path: %q", loc)
			}
		})
	}
}
