package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/freewhen/backend/internal/idprovider"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"

	"golang.org/x/oauth2"
)

// fakeGoogle is a shared httptest.Server stand-in for Google's OAuth +
// userinfo endpoints. Each test wires the relevant handlers and drives
// OAuthLogin / OAuthCallback through it.
type fakeGoogle struct {
	server *httptest.Server
	mux    *http.ServeMux

	// Configurable per-test response handlers.
	tokenHandler    http.HandlerFunc
	userInfoHandler http.HandlerFunc

	// Captured Authorization header from the userinfo endpoint so tests can
	// assert the bearer token was forwarded.
	lastUserInfoAuth string
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	g := &fakeGoogle{mux: http.NewServeMux()}
	g.mux.HandleFunc("/o/oauth2/v2/auth", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	g.mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if g.tokenHandler != nil {
			g.tokenHandler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"AT","token_type":"Bearer","expires_in":3600}`))
	})
	g.mux.HandleFunc("/oauth2/v3/userinfo", func(w http.ResponseWriter, r *http.Request) {
		g.lastUserInfoAuth = r.Header.Get("Authorization")
		if g.userInfoHandler != nil {
			g.userInfoHandler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sub":"google-123","email":"new@example.com","email_verified":true,"name":"New User"}`))
	})
	g.server = httptest.NewServer(g.mux)
	t.Cleanup(g.server.Close)
	return g
}

func (g *fakeGoogle) endpoint() oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:  g.server.URL + "/o/oauth2/v2/auth",
		TokenURL: g.server.URL + "/token",
	}
}

func (g *fakeGoogle) userInfoURL() string {
	return g.server.URL + "/oauth2/v3/userinfo"
}

func newGoogleTestAuthHandler(t *testing.T, st *fakeAuthStore, fg *fakeGoogle) *AuthHandler {
	t.Helper()
	googleProvider := idprovider.NewGoogle(
		"test-client", "test-secret", "https://app.test/auth/google/callback",
		idprovider.WithGoogleOAuthEndpoint(fg.endpoint()),
		idprovider.WithGoogleUserInfoURL(fg.userInfoURL()),
	)
	return &AuthHandler{
		store:        st,
		accounts:     service.NewAccounts(st),
		idProviders:  idprovider.NewRegistry(googleProvider),
		frontendURL:  "https://app.test",
		secureCookie: false,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// requestWithProvider returns a request whose ServeMux path-value for
// "provider" is set, mimicking how routing fills the param in production.
func requestWithProvider(method, target, provider string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.SetPathValue("provider", provider)
	return req
}

// emptyAuthHandler is the no-provider equivalent used for "not configured"
// checks. It still has a non-nil registry so the path-param lookup runs.
func emptyAuthHandler() *AuthHandler {
	return &AuthHandler{
		idProviders: idprovider.NewRegistry(),
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// Avoid unused-import warnings for json/model in trimmed test sets.
var (
	_ = json.Marshal
	_ = model.User{}
	_ = context.Background
)

func TestOAuthLogin_Redirects(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)

	req := requestWithProvider(http.MethodGet, "/auth/google?return_url=/calendars/abc", "google")
	rec := httptest.NewRecorder()
	h.OAuthLogin(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status: %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, fg.server.URL+"/o/oauth2/v2/auth") {
		t.Fatalf("location prefix: %q", loc)
	}
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatal("state missing")
	}
	var s oauthState
	if err := json.Unmarshal([]byte(state), &s); err != nil {
		t.Fatalf("state decode: %v", err)
	}
	if s.ReturnURL != "/calendars/abc" {
		t.Fatalf("return_url: %q", s.ReturnURL)
	}
	// Nonce cookie set and scoped to the provider callback path.
	var nonce *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oauth_nonce" {
			nonce = c
		}
	}
	if nonce == nil || nonce.Value != s.Nonce {
		t.Fatalf("nonce cookie missing or mismatched")
	}
	if nonce.Path != "/auth/google/callback" {
		t.Errorf("nonce cookie path = %q, want /auth/google/callback", nonce.Path)
	}
}

func TestOAuthLogin_AbsoluteReturnURL_FallsBack(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)

	req := requestWithProvider(http.MethodGet, "/auth/google?return_url=https://evil/x", "google")
	rec := httptest.NewRecorder()
	h.OAuthLogin(rec, req)

	loc := rec.Header().Get("Location")
	parsed, _ := url.Parse(loc)
	var s oauthState
	_ = json.Unmarshal([]byte(parsed.Query().Get("state")), &s)
	if s.ReturnURL != "/" {
		t.Fatalf("return_url not sanitised: %q", s.ReturnURL)
	}
}

func TestOAuthLogin_UnknownProvider(t *testing.T) {
	h := emptyAuthHandler()
	req := requestWithProvider(http.MethodGet, "/auth/nope", "nope")
	rec := httptest.NewRecorder()
	h.OAuthLogin(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestOAuthLogin_ProviderUnconfigured(t *testing.T) {
	// Provider exists in the registry but has no OAuth credentials.
	h := &AuthHandler{
		idProviders: idprovider.NewRegistry(idprovider.NewGoogle("", "", "")),
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	req := requestWithProvider(http.MethodGet, "/auth/google", "google")
	rec := httptest.NewRecorder()
	h.OAuthLogin(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", rec.Code)
	}
}

// driveLoginThenCallback runs OAuthLogin, captures the state + nonce cookie,
// then issues a callback request with those values plus the supplied code.
func driveLoginThenCallback(t *testing.T, h *AuthHandler, code string) *httptest.ResponseRecorder {
	t.Helper()
	loginReq := requestWithProvider(http.MethodGet, "/auth/google?return_url=/dashboard", "google")
	loginRec := httptest.NewRecorder()
	h.OAuthLogin(loginRec, loginReq)
	loc, _ := url.Parse(loginRec.Header().Get("Location"))
	state := loc.Query().Get("state")

	var nonce *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == "oauth_nonce" {
			nonce = c
		}
	}
	if nonce == nil {
		t.Fatal("no nonce cookie")
	}

	cbReq := requestWithProvider(http.MethodGet,
		"/auth/google/callback?code="+code+"&state="+url.QueryEscape(state), "google")
	cbReq.AddCookie(nonce)
	cbRec := httptest.NewRecorder()
	h.OAuthCallback(cbRec, cbReq)
	return cbRec
}

func TestOAuthCallback_HappyPath_NewUser(t *testing.T) {
	fg := newFakeGoogle(t)
	st := newFakeAuthStore()
	h := newGoogleTestAuthHandler(t, st, fg)

	rec := driveLoginThenCallback(t, h, "abc")
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "https://app.test/dashboard" {
		t.Fatalf("redirect: %q", loc)
	}
	if fg.lastUserInfoAuth == "" || !strings.HasPrefix(fg.lastUserInfoAuth, "Bearer ") {
		t.Fatalf("userinfo Authorization header: %q", fg.lastUserInfoAuth)
	}
	if u, ok := st.usersByPID["google:google-123"]; !ok || u.Email != "new@example.com" {
		t.Fatalf("user not created via provider id: %+v", u)
	}
	var sess *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			sess = c
		}
	}
	if sess == nil {
		t.Fatal("session cookie not set")
	}
}

func TestOAuthCallback_ReturningUser_NoDuplicate(t *testing.T) {
	fg := newFakeGoogle(t)
	st := newFakeAuthStore()
	existing := model.User{ID: "u_old", Email: "old@example.com", Provider: "google", ProviderID: "google-123"}
	st.usersByPID["google:google-123"] = existing
	st.usersByID["u_old"] = existing
	h := newGoogleTestAuthHandler(t, st, fg)

	rec := driveLoginThenCallback(t, h, "abc")
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status: %d", rec.Code)
	}
	var sess *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			sess = c
		}
	}
	if sess == nil || sess.Value != "sess_u_old" {
		t.Fatalf("session not bound to existing user: %+v", sess)
	}
}

func TestOAuthCallback_EmailCollision_LinksByEmail(t *testing.T) {
	fg := newFakeGoogle(t)
	st := newFakeAuthStore()
	// User signed up via magic link first — same email, no provider id.
	existing := model.User{ID: "u_email", Email: "new@example.com", Provider: "email"}
	st.usersByEmail["new@example.com"] = existing
	st.usersByID["u_email"] = existing
	h := newGoogleTestAuthHandler(t, st, fg)

	rec := driveLoginThenCallback(t, h, "abc")
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	// Current behaviour: returns the existing email-provider user (no upgrade,
	// no duplicate insert). Locked in by this test.
	var sess *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			sess = c
		}
	}
	if sess == nil || sess.Value != "sess_u_email" {
		t.Fatalf("session not bound to existing email user: %+v", sess)
	}
}

// unverifiedUserInfo serves a userinfo payload whose email isn't verified.
func unverifiedUserInfo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"sub":"google-123","email":"new@example.com","email_verified":false,"name":"New User"}`)
}

func assertNoSession(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" && c.MaxAge >= 0 && c.Value != "" {
			t.Fatalf("session cookie set: %+v", c)
		}
	}
}

func TestOAuthCallback_UnverifiedEmail_DoesNotLinkByEmail(t *testing.T) {
	fg := newFakeGoogle(t)
	fg.userInfoHandler = unverifiedUserInfo
	st := newFakeAuthStore()
	existing := model.User{ID: "u_email", Email: "new@example.com", Provider: "email"}
	st.usersByEmail["new@example.com"] = existing
	st.usersByID["u_email"] = existing
	h := newGoogleTestAuthHandler(t, st, fg)

	rec := driveLoginThenCallback(t, h, "abc")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "email_not_verified") {
		t.Errorf("body missing email_not_verified: %s", rec.Body.String())
	}
	assertNoSession(t, rec)
}

func TestOAuthCallback_UnverifiedEmail_DoesNotCreateUser(t *testing.T) {
	fg := newFakeGoogle(t)
	fg.userInfoHandler = unverifiedUserInfo
	st := newFakeAuthStore()
	h := newGoogleTestAuthHandler(t, st, fg)

	rec := driveLoginThenCallback(t, h, "abc")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	if len(st.usersByPID) != 0 || len(st.usersByEmail) != 0 {
		t.Errorf("user created despite unverified email: %+v %+v", st.usersByPID, st.usersByEmail)
	}
	assertNoSession(t, rec)
}

func TestOAuthCallback_UnverifiedEmail_ExistingProviderLinkStillWorks(t *testing.T) {
	fg := newFakeGoogle(t)
	fg.userInfoHandler = unverifiedUserInfo
	st := newFakeAuthStore()
	existing := model.User{ID: "u_old", Email: "old@example.com", Provider: "google", ProviderID: "google-123"}
	st.usersByPID["google:google-123"] = existing
	st.usersByID["u_old"] = existing
	h := newGoogleTestAuthHandler(t, st, fg)

	rec := driveLoginThenCallback(t, h, "abc")
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOAuthCallback_BadStateJSON(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	req := requestWithProvider(http.MethodGet, "/auth/google/callback?code=x&state=not-json", "google")
	rec := httptest.NewRecorder()
	h.OAuthCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestOAuthCallback_NonceMismatch(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	state, _ := json.Marshal(oauthState{Nonce: "expected", ReturnURL: "/"})
	req := requestWithProvider(http.MethodGet,
		"/auth/google/callback?code=x&state="+url.QueryEscape(string(state)), "google")
	req.AddCookie(&http.Cookie{Name: "oauth_nonce", Value: "different"})
	rec := httptest.NewRecorder()
	h.OAuthCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestOAuthCallback_MissingCode(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	state, _ := json.Marshal(oauthState{Nonce: "n", ReturnURL: "/"})
	req := requestWithProvider(http.MethodGet,
		"/auth/google/callback?state="+url.QueryEscape(string(state)), "google")
	req.AddCookie(&http.Cookie{Name: "oauth_nonce", Value: "n"})
	rec := httptest.NewRecorder()
	h.OAuthCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestOAuthCallback_TokenExchangeFails(t *testing.T) {
	fg := newFakeGoogle(t)
	fg.tokenHandler = func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	rec := driveLoginThenCallback(t, h, "abc")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			t.Fatal("session cookie set despite exchange failure")
		}
	}
}

func TestOAuthCallback_UserInfoFails(t *testing.T) {
	fg := newFakeGoogle(t)
	fg.userInfoHandler = func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	rec := driveLoginThenCallback(t, h, "abc")
	// Identify now returns an error on non-2xx userinfo responses, so the
	// callback short-circuits with 500. (Previously the handler silently
	// decoded zero values and proceeded — the seam lets us tighten this.)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			t.Fatal("session cookie set despite userinfo failure")
		}
	}
}

func TestOAuthCallback_UserInfoMalformedJSON(t *testing.T) {
	fg := newFakeGoogle(t)
	fg.userInfoHandler = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{not json`)
	}
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	rec := driveLoginThenCallback(t, h, "abc")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestOAuthCallback_UnknownProvider(t *testing.T) {
	h := emptyAuthHandler()
	req := requestWithProvider(http.MethodGet, "/auth/nope/callback", "nope")
	rec := httptest.NewRecorder()
	h.OAuthCallback(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: %d", rec.Code)
	}
}
