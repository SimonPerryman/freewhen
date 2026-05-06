package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	sessionOrder   []string // creation order, oldest first
	magicLinks     map[string]magicLinkRecord // plain token -> record
	magicLinkSeq   int
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
	if _, exists := f.sessions[tok]; exists {
		tok = fmt.Sprintf("sess_%s_%d", userID, len(f.sessionOrder))
	}
	f.sessions[tok] = &model.Session{UserID: userID, ExpiresAt: time.Now().Add(24 * time.Hour)}
	f.sessionOrder = append(f.sessionOrder, tok)
	return tok, nil
}

func (f *fakeAuthStore) PruneSessionsForUser(_ context.Context, userID string, keep int) error {
	// Walk newest-first, keep the first `keep` sessions for this user, delete the rest.
	seen := 0
	kept := make([]string, 0, len(f.sessionOrder))
	for i := len(f.sessionOrder) - 1; i >= 0; i-- {
		tok := f.sessionOrder[i]
		s, ok := f.sessions[tok]
		if !ok {
			continue
		}
		if s.UserID == userID {
			if seen >= keep {
				delete(f.sessions, tok)
				continue
			}
			seen++
		}
		kept = append([]string{tok}, kept...)
	}
	f.sessionOrder = kept
	return nil
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
	for i, t := range f.sessionOrder {
		if t == token {
			f.sessionOrder = append(f.sessionOrder[:i], f.sessionOrder[i+1:]...)
			break
		}
	}
	return nil
}

func (f *fakeAuthStore) CreateMagicLinkToken(_ context.Context, email, returnURL string) (string, error) {
	if f.createMagicErr != nil {
		return "", f.createMagicErr
	}
	// Mirror the real store: invalidate any prior outstanding tokens for this
	// email before issuing a new one.
	for k, rec := range f.magicLinks {
		if rec.email == email {
			delete(f.magicLinks, k)
		}
	}
	f.magicLinkSeq++
	tok := fmt.Sprintf("mltok_%s_%d", email, f.magicLinkSeq)
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
		magicLimiter: newMagicLinkLimiter(),
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

	// Now drive the verify endpoint with the captured token.
	rawToken := strings.TrimPrefix(sender.calls[0].link, "https://app.test/auth/magic-link/callback?token=")
	cbRec := postVerify(h, rawToken)

	if cbRec.Code != http.StatusOK {
		t.Fatalf("verify status: got %d, want 200", cbRec.Code)
	}
	if got := decodeReturnURL(t, cbRec); got != "/calendars/abc" {
		t.Fatalf("return_url: got %q, want %q", got, "/calendars/abc")
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
	cbRec := postVerify(h, rawToken)
	if cbRec.Code != http.StatusOK {
		t.Fatalf("verify status: %d", cbRec.Code)
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
	cbRec := postVerify(h, rawToken)
	if cbRec.Code != http.StatusOK {
		t.Fatalf("verify status: %d", cbRec.Code)
	}
	if got := decodeReturnURL(t, cbRec); got != "/" {
		t.Fatalf("return_url: got %q, want fallback to /", got)
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

func TestMagicLinkVerify_MissingToken(t *testing.T) {
	h := newTestAuthHandler(t, newFakeAuthStore(), &fakeEmailSender{})
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link/verify", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.MagicLinkVerify(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestMagicLinkVerify_InvalidJSON(t *testing.T) {
	h := newTestAuthHandler(t, newFakeAuthStore(), &fakeEmailSender{})
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link/verify", strings.NewReader(`{not json`))
	rec := httptest.NewRecorder()
	h.MagicLinkVerify(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestMagicLinkVerify_UnknownToken(t *testing.T) {
	h := newTestAuthHandler(t, newFakeAuthStore(), &fakeEmailSender{})
	rec := postVerify(h, "nope")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

// magicLinkRequest issues a SendMagicLink request with the given email and
// optional override of the request remote IP via X-Forwarded-For.
func sendMagicLinkRequest(t *testing.T, h *AuthHandler, email, ip string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"email":"` + email + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link", body)
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
	}
	rec := httptest.NewRecorder()
	h.SendMagicLink(rec, req)
	return rec
}

func TestSendMagicLink_RateLimitedPerEmail(t *testing.T) {
	st := newFakeAuthStore()
	sender := &fakeEmailSender{}
	h := newTestAuthHandler(t, st, sender)

	// Use a different IP per request so the per-IP budget doesn't trip first.
	for i := 0; i < magicLinkPerEmailMax; i++ {
		ip := "10.0.0." + string(rune('1'+i))
		rec := sendMagicLinkRequest(t, h, "victim@example.com", ip)
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d: got %d, want 200", i, rec.Code)
		}
	}

	rec := sendMagicLinkRequest(t, h, "victim@example.com", "10.0.0.99")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit attempt: got %d, want 429", rec.Code)
	}
	if len(sender.calls) != magicLinkPerEmailMax {
		t.Fatalf("emails sent: got %d, want %d", len(sender.calls), magicLinkPerEmailMax)
	}

	// A different email from the same string of IPs is independent.
	rec = sendMagicLinkRequest(t, h, "other@example.com", "10.0.0.50")
	if rec.Code != http.StatusOK {
		t.Fatalf("different email blocked: got %d", rec.Code)
	}
}

func TestSendMagicLink_RateLimitedPerIP(t *testing.T) {
	st := newFakeAuthStore()
	sender := &fakeEmailSender{}
	h := newTestAuthHandler(t, st, sender)

	// Different email each time so per-email budget never trips.
	for i := 0; i < magicLinkPerIPMax; i++ {
		email := "u" + string(rune('a'+i)) + "@example.com"
		rec := sendMagicLinkRequest(t, h, email, "203.0.113.1")
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d (%s): got %d, want 200", i, email, rec.Code)
		}
	}

	rec := sendMagicLinkRequest(t, h, "fresh@example.com", "203.0.113.1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit attempt: got %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Fatalf("missing Retry-After header on 429")
	}

	// A request from a different IP (different email) is independent.
	rec = sendMagicLinkRequest(t, h, "elsewhere@example.com", "198.51.100.7")
	if rec.Code != http.StatusOK {
		t.Fatalf("different IP blocked: got %d", rec.Code)
	}
}

// TestSendMagicLink_AlwaysIssuesEvenForUnknownUser pins the F6 invariant
// (see the docstring on SendMagicLink). The handler must call
// CreateMagicLinkToken regardless of whether the email maps to an existing
// user — that's what keeps known-vs-unknown response timing uniform.
func TestSendMagicLink_AlwaysIssuesEvenForUnknownUser(t *testing.T) {
	st := newFakeAuthStore()
	// No users seeded.
	if len(st.usersByEmail) != 0 {
		t.Fatalf("precondition: expected no users")
	}
	sender := &fakeEmailSender{}
	h := newTestAuthHandler(t, st, sender)

	rec := sendMagicLinkRequest(t, h, "ghost@example.com", "10.0.0.1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if len(st.magicLinks) != 1 {
		t.Fatalf("expected token issued for unknown email, got %d", len(st.magicLinks))
	}
	if len(sender.calls) != 1 {
		t.Fatalf("expected email sent for unknown email, got %d", len(sender.calls))
	}
}

func TestMagicLinkVerify_NonRelativeReturnURL_FallsBack(t *testing.T) {
	st := newFakeAuthStore()
	// Inject a token directly with a stored absolute return_url to confirm
	// verify re-validates rather than trusting whatever was stored.
	st.magicLinks["forged"] = magicLinkRecord{email: "x@example.com", returnURL: "https://evil/"}
	h := newTestAuthHandler(t, st, &fakeEmailSender{})

	rec := postVerify(h, "forged")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if got := decodeReturnURL(t, rec); got != "/" {
		t.Fatalf("return_url: got %q, want fallback to /", got)
	}
}

// TestMagicLinkVerify_PrunesOldSessions asserts the login path caps the
// per-user session count at maxConcurrentSessionsPerUser. Auth audit F11.
func TestMagicLinkVerify_PrunesOldSessions(t *testing.T) {
	st := newFakeAuthStore()
	h := newTestAuthHandler(t, st, &fakeEmailSender{})

	// Drive 12 logins for the same email by pre-seeding magic-link records
	// directly (bypasses the per-email rate limit on SendMagicLink, which
	// isn't what this test is exercising).
	for i := 0; i < 12; i++ {
		tok := fmt.Sprintf("preseed_%d", i)
		st.magicLinks[tok] = magicLinkRecord{email: "carol@example.com", returnURL: "/"}
		rec := postVerify(h, tok)
		if rec.Code != http.StatusOK {
			t.Fatalf("login %d: status %d", i, rec.Code)
		}
	}

	user := st.usersByEmail["carol@example.com"]
	count := 0
	for _, s := range st.sessions {
		if s.UserID == user.ID {
			count++
		}
	}
	if count != maxConcurrentSessionsPerUser {
		t.Fatalf("expected %d sessions after 12 logins, got %d", maxConcurrentSessionsPerUser, count)
	}
}

// postVerify drives MagicLinkVerify with the given token.
func postVerify(h *AuthHandler, token string) *httptest.ResponseRecorder {
	body := strings.NewReader(`{"token":"` + token + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link/verify", body)
	rec := httptest.NewRecorder()
	h.MagicLinkVerify(rec, req)
	return rec
}

func decodeReturnURL(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp magicLinkVerifyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%q)", err, rec.Body.String())
	}
	return resp.ReturnURL
}
