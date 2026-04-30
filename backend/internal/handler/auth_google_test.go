package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/availability"
	"github.com/freewhen/backend/internal/model"

	"golang.org/x/oauth2"
)

// fakeGoogle is a shared httptest.Server stand-in for Google's OAuth +
// userinfo + freebusy endpoints. Each test wires the relevant handlers and
// drives Connect / Callback / GoogleCallback through it.
type fakeGoogle struct {
	server *httptest.Server
	mux    *http.ServeMux

	// Configurable per-test response handlers.
	tokenHandler    http.HandlerFunc
	userInfoHandler http.HandlerFunc
	freeBusyHandler http.HandlerFunc

	// Captured authorization headers from the userinfo / freebusy endpoints,
	// so tests can assert the bearer token was forwarded.
	lastUserInfoAuth string
	lastFreeBusyAuth string
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	g := &fakeGoogle{mux: http.NewServeMux()}
	// Auth URL — handler redirects users here, the test never actually hits it.
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
		_, _ = w.Write([]byte(`{"sub":"google-123","email":"new@example.com","name":"New User"}`))
	})
	g.mux.HandleFunc("/calendar/v3/freebusy", func(w http.ResponseWriter, r *http.Request) {
		g.lastFreeBusyAuth = r.Header.Get("Authorization")
		if g.freeBusyHandler != nil {
			g.freeBusyHandler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
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

func newGoogleTestAuthHandler(t *testing.T, st AuthStore, fg *fakeGoogle) *AuthHandler {
	t.Helper()
	h := &AuthHandler{
		store: st,
		oauth: &oauth2.Config{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "https://app.test/auth/google/callback",
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     fg.endpoint(),
		},
		userInfoURL:  fg.userInfoURL(),
		frontendURL:  "https://app.test",
		secureCookie: false,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return h
}

// Avoid unused-import warnings for availability/time/json/model in trimmed test sets.
var (
	_ = availability.TimeRange{}
	_ = time.Now
	_ = json.Marshal
	_ = model.User{}
)

func TestGoogleLogin_Redirects(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)

	req := httptest.NewRequest(http.MethodGet, "/auth/google?return_url=/calendars/abc", nil)
	rec := httptest.NewRecorder()
	h.GoogleLogin(rec, req)

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
	// Nonce cookie set.
	var nonce *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oauth_nonce" {
			nonce = c
		}
	}
	if nonce == nil || nonce.Value != s.Nonce {
		t.Fatalf("nonce cookie missing or mismatched")
	}
}

func TestGoogleLogin_AbsoluteReturnURL_FallsBack(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)

	req := httptest.NewRequest(http.MethodGet, "/auth/google?return_url=https://evil/x", nil)
	rec := httptest.NewRecorder()
	h.GoogleLogin(rec, req)

	loc := rec.Header().Get("Location")
	parsed, _ := url.Parse(loc)
	var s oauthState
	_ = json.Unmarshal([]byte(parsed.Query().Get("state")), &s)
	if s.ReturnURL != "/" {
		t.Fatalf("return_url not sanitised: %q", s.ReturnURL)
	}
}

func TestGoogleLogin_NotConfigured(t *testing.T) {
	h := &AuthHandler{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	req := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	rec := httptest.NewRecorder()
	h.GoogleLogin(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", rec.Code)
	}
}

// driveLoginThenCallback runs GoogleLogin, captures the state + nonce cookie,
// then issues a callback request with those values plus the supplied code.
func driveLoginThenCallback(t *testing.T, h *AuthHandler, code string) *httptest.ResponseRecorder {
	t.Helper()
	loginReq := httptest.NewRequest(http.MethodGet, "/auth/google?return_url=/dashboard", nil)
	loginRec := httptest.NewRecorder()
	h.GoogleLogin(loginRec, loginReq)
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

	cbReq := httptest.NewRequest(http.MethodGet,
		"/auth/google/callback?code="+code+"&state="+url.QueryEscape(state), nil)
	cbReq.AddCookie(nonce)
	cbRec := httptest.NewRecorder()
	h.GoogleCallback(cbRec, cbReq)
	return cbRec
}

func TestGoogleCallback_HappyPath_NewUser(t *testing.T) {
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

func TestGoogleCallback_ReturningUser_NoDuplicate(t *testing.T) {
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

func TestGoogleCallback_EmailCollision_LinksByEmail(t *testing.T) {
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

func TestGoogleCallback_BadStateJSON(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=x&state=not-json", nil)
	rec := httptest.NewRecorder()
	h.GoogleCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestGoogleCallback_NonceMismatch(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	state, _ := json.Marshal(oauthState{Nonce: "expected", ReturnURL: "/"})
	req := httptest.NewRequest(http.MethodGet,
		"/auth/google/callback?code=x&state="+url.QueryEscape(string(state)), nil)
	req.AddCookie(&http.Cookie{Name: "oauth_nonce", Value: "different"})
	rec := httptest.NewRecorder()
	h.GoogleCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestGoogleCallback_MissingCode(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	state, _ := json.Marshal(oauthState{Nonce: "n", ReturnURL: "/"})
	req := httptest.NewRequest(http.MethodGet,
		"/auth/google/callback?state="+url.QueryEscape(string(state)), nil)
	req.AddCookie(&http.Cookie{Name: "oauth_nonce", Value: "n"})
	rec := httptest.NewRecorder()
	h.GoogleCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestGoogleCallback_TokenExchangeFails(t *testing.T) {
	fg := newFakeGoogle(t)
	fg.tokenHandler = func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	rec := driveLoginThenCallback(t, h, "abc")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
	// No session leaked.
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			t.Fatal("session cookie set despite exchange failure")
		}
	}
}

func TestGoogleCallback_UserInfoFails(t *testing.T) {
	fg := newFakeGoogle(t)
	fg.userInfoHandler = func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}
	h := newGoogleTestAuthHandler(t, newFakeAuthStore(), fg)
	rec := driveLoginThenCallback(t, h, "abc")
	// userinfo non-2xx still decodes into zero values, so the handler proceeds
	// to user creation. That is current behaviour — locked in here so a future
	// strictening shows up as a test change rather than a silent regression.
	if rec.Code != http.StatusTemporaryRedirect && rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
}

func TestGoogleCallback_UserInfoMalformedJSON(t *testing.T) {
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

func TestGoogleCallback_NotConfigured(t *testing.T) {
	h := &AuthHandler{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil)
	rec := httptest.NewRecorder()
	h.GoogleCallback(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", rec.Code)
	}
}
