package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"
)

// fakeAuthStore is a minimal in-memory AuthStore for handler tests.
type fakeAuthStore struct {
	usersByEmail   map[string]model.User
	usersByID      map[string]model.User
	usersByPID     map[string]model.User // key: provider+":"+providerID
	sessions       map[string]*model.Session
	magicLinks     map[string]magicLinkRecord // plain token -> record
	createUserErr  error
	createSessErr  error
	verifyMagicErr error
	createMagicErr error
}

type magicLinkRecord struct {
	email     string
	returnURL string
	used      bool
}

func newFakeAuthStore() *fakeAuthStore {
	return &fakeAuthStore{
		usersByEmail: map[string]model.User{},
		usersByID:    map[string]model.User{},
		usersByPID:   map[string]model.User{},
		sessions:     map[string]*model.Session{},
		magicLinks:   map[string]magicLinkRecord{},
	}
}

func (f *fakeAuthStore) CreateUser(_ context.Context, email, name, provider, providerID string) (model.User, error) {
	if f.createUserErr != nil {
		return model.User{}, f.createUserErr
	}
	u := model.User{
		ID:         "u_" + email + "_" + provider,
		Email:      email,
		Name:       name,
		Provider:   provider,
		ProviderID: providerID,
		CreatedAt:  time.Now(),
	}
	f.usersByEmail[email] = u
	f.usersByID[u.ID] = u
	if providerID != "" {
		f.usersByPID[provider+":"+providerID] = u
	}
	return u, nil
}

func (f *fakeAuthStore) GetUser(_ context.Context, id string) (model.User, error) {
	u, ok := f.usersByID[id]
	if !ok {
		return model.User{}, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeAuthStore) FindUserByEmail(_ context.Context, email string) (model.User, error) {
	u, ok := f.usersByEmail[email]
	if !ok {
		return model.User{}, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeAuthStore) FindUserByProviderID(_ context.Context, provider, providerID string) (model.User, error) {
	u, ok := f.usersByPID[provider+":"+providerID]
	if !ok {
		return model.User{}, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeAuthStore) UpdateUserName(_ context.Context, userID, name string) (model.User, error) {
	u, ok := f.usersByID[userID]
	if !ok {
		return model.User{}, store.ErrNotFound
	}
	u.Name = name
	f.usersByID[userID] = u
	f.usersByEmail[u.Email] = u
	return u, nil
}

func (f *fakeAuthStore) CreateSession(_ context.Context, userID string) (string, error) {
	if f.createSessErr != nil {
		return "", f.createSessErr
	}
	tok := "sess_" + userID
	f.sessions[tok] = &model.Session{UserID: userID, ExpiresAt: time.Now().Add(24 * time.Hour)}
	return tok, nil
}

func (f *fakeAuthStore) GetSession(_ context.Context, token string) (*model.Session, error) {
	s, ok := f.sessions[token]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (f *fakeAuthStore) DeleteSession(_ context.Context, token string) error {
	delete(f.sessions, token)
	return nil
}

func (f *fakeAuthStore) CreateMagicLinkToken(_ context.Context, email, returnURL string) (string, error) {
	if f.createMagicErr != nil {
		return "", f.createMagicErr
	}
	tok := "mltok_" + email
	f.magicLinks[tok] = magicLinkRecord{email: email, returnURL: returnURL}
	return tok, nil
}

func (f *fakeAuthStore) VerifyMagicLinkToken(_ context.Context, plainToken string) (string, string, error) {
	if f.verifyMagicErr != nil {
		return "", "", f.verifyMagicErr
	}
	rec, ok := f.magicLinks[plainToken]
	if !ok || rec.used {
		return "", "", store.ErrNotFound
	}
	rec.used = true
	f.magicLinks[plainToken] = rec
	return rec.email, rec.returnURL, nil
}

// fakeEmailSender captures sent links for assertion.
type fakeEmailSender struct {
	calls []struct{ to, link string }
	err   error
}

func (f *fakeEmailSender) SendMagicLink(to, link string) error {
	f.calls = append(f.calls, struct{ to, link string }{to, link})
	return f.err
}

func newTestAuthHandler(t *testing.T, st AuthStore, sender EmailSender) *AuthHandler {
	t.Helper()
	return &AuthHandler{
		store:        st,
		email:        sender,
		frontendURL:  "https://app.test",
		secureCookie: false,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestSendMagicLink_HappyPath(t *testing.T) {
	st := newFakeAuthStore()
	sender := &fakeEmailSender{}
	h := newTestAuthHandler(t, st, sender)

	body := strings.NewReader(`{"email":"alice@example.com","return_url":"/calendars/abc"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", body)
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if len(sender.calls) != 1 {
		t.Fatalf("sender calls: got %d, want 1", len(sender.calls))
	}
	if sender.calls[0].to != "alice@example.com" {
		t.Fatalf("to: got %q", sender.calls[0].to)
	}
	if !strings.Contains(sender.calls[0].link, "token=mltok_alice@example.com") {
		t.Fatalf("link missing token: %q", sender.calls[0].link)
	}
	if !strings.HasPrefix(sender.calls[0].link, "https://app.test/auth/magic-link/callback?token=") {
		t.Fatalf("link prefix: got %q", sender.calls[0].link)
	}

	// Now drive the callback with the captured token.
	rawToken := strings.TrimPrefix(sender.calls[0].link, "https://app.test/auth/magic-link/callback?token=")
	cbReq := httptest.NewRequest(http.MethodGet, "/auth/magic-link/callback?token="+rawToken, nil)
	cbRec := httptest.NewRecorder()
	h.MagicLinkCallback(cbRec, cbReq)

	if cbRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("callback status: got %d, want 307", cbRec.Code)
	}
	loc := cbRec.Header().Get("Location")
	if loc != "https://app.test/calendars/abc" {
		t.Fatalf("redirect: got %q", loc)
	}
	// Session cookie should be set.
	var sessCookie *http.Cookie
	for _, c := range cbRec.Result().Cookies() {
		if c.Name == "session" {
			sessCookie = c
		}
	}
	if sessCookie == nil || sessCookie.Value == "" {
		t.Fatalf("session cookie not set")
	}
	if _, ok := st.usersByEmail["alice@example.com"]; !ok {
		t.Fatalf("user not created")
	}
}

func TestSendMagicLink_ReturningUser_NoDuplicate(t *testing.T) {
	st := newFakeAuthStore()
	// Pre-existing user from prior magic-link signup.
	existing := model.User{ID: "u_existing", Email: "bob@example.com", Provider: "email"}
	st.usersByEmail["bob@example.com"] = existing
	st.usersByID["u_existing"] = existing

	sender := &fakeEmailSender{}
	h := newTestAuthHandler(t, st, sender)

	body := strings.NewReader(`{"email":"bob@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", body)
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}

	rawToken := strings.TrimPrefix(sender.calls[0].link, "https://app.test/auth/magic-link/callback?token=")
	cbReq := httptest.NewRequest(http.MethodGet, "/auth/magic-link/callback?token="+rawToken, nil)
	cbRec := httptest.NewRecorder()
	h.MagicLinkCallback(cbRec, cbReq)
	if cbRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("callback status: %d", cbRec.Code)
	}
	// Session belongs to the existing user, no duplicate insert.
	var sessCookie *http.Cookie
	for _, c := range cbRec.Result().Cookies() {
		if c.Name == "session" {
			sessCookie = c
		}
	}
	if sessCookie == nil || sessCookie.Value != "sess_u_existing" {
		t.Fatalf("session not bound to existing user: %+v", sessCookie)
	}
}

func TestSendMagicLink_InvalidEmail(t *testing.T) {
	cases := []string{
		`{"email":""}`,
		`{"email":"not-an-email"}`,
		`{"email":"missing-tld@host"}`,
	}
	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			st := newFakeAuthStore()
			sender := &fakeEmailSender{}
			h := newTestAuthHandler(t, st, sender)
			req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", strings.NewReader(body))
			rec := httptest.NewRecorder()
			h.SendMagicLink(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want 400 for %s", rec.Code, body)
			}
			if len(sender.calls) != 0 {
				t.Fatalf("email sent for invalid request")
			}
		})
	}
}

func TestSendMagicLink_InvalidJSON(t *testing.T) {
	h := newTestAuthHandler(t, newFakeAuthStore(), &fakeEmailSender{})
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", bytes.NewReader([]byte("{not json")))
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestSendMagicLink_NoEmailSenderConfigured(t *testing.T) {
	h := newTestAuthHandler(t, newFakeAuthStore(), nil)
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", strings.NewReader(`{"email":"a@b.co"}`))
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestSendMagicLink_NonRelativeReturnURL_FallsBack(t *testing.T) {
	st := newFakeAuthStore()
	sender := &fakeEmailSender{}
	h := newTestAuthHandler(t, st, sender)

	body := strings.NewReader(`{"email":"c@example.com","return_url":"https://evil.example.com/x"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", body)
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}

	rawToken := strings.TrimPrefix(sender.calls[0].link, "https://app.test/auth/magic-link/callback?token=")
	cbReq := httptest.NewRequest(http.MethodGet, "/auth/magic-link/callback?token="+rawToken, nil)
	cbRec := httptest.NewRecorder()
	h.MagicLinkCallback(cbRec, cbReq)
	if loc := cbRec.Header().Get("Location"); loc != "https://app.test/" {
		t.Fatalf("redirect: got %q, want fallback to /", loc)
	}
}

func TestSendMagicLink_EmailSenderFailure_DoesNotLeak(t *testing.T) {
	st := newFakeAuthStore()
	sender := &fakeEmailSender{err: errors.New("resend down")}
	h := newTestAuthHandler(t, st, sender)

	body := strings.NewReader(`{"email":"d@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", body)
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)
	// Anti-enumeration: surface as 200 so callers can't distinguish sender failure
	// from "email doesn't exist".
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
}

func TestMagicLinkCallback_MissingToken(t *testing.T) {
	h := newTestAuthHandler(t, newFakeAuthStore(), &fakeEmailSender{})
	req := httptest.NewRequest(http.MethodGet, "/auth/magic-link/callback", nil)
	rec := httptest.NewRecorder()
	h.MagicLinkCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestMagicLinkCallback_UnknownToken(t *testing.T) {
	h := newTestAuthHandler(t, newFakeAuthStore(), &fakeEmailSender{})
	req := httptest.NewRequest(http.MethodGet, "/auth/magic-link/callback?token=nope", nil)
	rec := httptest.NewRecorder()
	h.MagicLinkCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestMagicLinkCallback_NonRelativeReturnURL_FallsBack(t *testing.T) {
	st := newFakeAuthStore()
	// Inject a token directly with a stored absolute return_url to confirm
	// the callback re-validates rather than trusting whatever was stored.
	st.magicLinks["forged"] = magicLinkRecord{email: "x@example.com", returnURL: "https://evil/"}
	h := newTestAuthHandler(t, st, &fakeEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/auth/magic-link/callback?token=forged", nil)
	rec := httptest.NewRecorder()
	h.MagicLinkCallback(rec, req)
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status: %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://app.test/" {
		t.Fatalf("redirect: got %q, want https://app.test/", loc)
	}
}
