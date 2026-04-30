package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/availability"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/ratelimit"
	"github.com/freewhen/backend/internal/store"

	"golang.org/x/oauth2"
)

// fakeCalIntegrationStore satisfies CalendarIntegrationStore.
type fakeCalIntegrationStore struct {
	consentAckCalls []string // user IDs
	consentAckErr   error
}

func (f *fakeCalIntegrationStore) SetCalendarImportConsentAck(_ context.Context, userID string) error {
	if f.consentAckErr != nil {
		return f.consentAckErr
	}
	f.consentAckCalls = append(f.consentAckCalls, userID)
	return nil
}

func newCalIntegrationHandler(t *testing.T, st CalendarIntegrationStore, cal CalendarService, fg *fakeGoogle, fb FreeBusyFetcher) *CalendarIntegrationHandler {
	t.Helper()
	if fb == nil {
		fb = func(_ context.Context, _ string, _, _ time.Time, _ string) ([]availability.TimeRange, error) {
			return nil, nil
		}
	}
	h := &CalendarIntegrationHandler{
		store: st,
		cal:   cal,
		oauth: &oauth2.Config{
			ClientID:     "test",
			ClientSecret: "secret",
			RedirectURL:  "https://app.test/auth/google/calendar/callback",
			Scopes:       []string{"https://www.googleapis.com/auth/calendar.freebusy"},
			Endpoint:     fg.endpoint(),
		},
		frontendURL:  "https://app.test",
		secureCookie: false,
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		limiter:      ratelimit.New(10, time.Hour),
		freeBusy:     fb,
	}
	return h
}

// --- ConsentAck ---

func TestConsentAck_Unauthenticated(t *testing.T) {
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, newFakeGoogle(t), nil)
	req := httptest.NewRequest(http.MethodPost, "/auth/google/calendar/consent-ack", nil)
	rec := httptest.NewRecorder()
	h.ConsentAck(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestConsentAck_HappyPath(t *testing.T) {
	st := &fakeCalIntegrationStore{}
	h := newCalIntegrationHandler(t, st, &mockService{}, newFakeGoogle(t), nil)
	req := withUser(httptest.NewRequest(http.MethodPost, "/auth/google/calendar/consent-ack", nil), testUser)
	rec := httptest.NewRecorder()
	h.ConsentAck(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: %d", rec.Code)
	}
	if len(st.consentAckCalls) != 1 || st.consentAckCalls[0] != "u1" {
		t.Fatalf("consent ack calls: %+v", st.consentAckCalls)
	}
}

func TestConsentAck_StoreFailure(t *testing.T) {
	st := &fakeCalIntegrationStore{consentAckErr: errors.New("db down")}
	h := newCalIntegrationHandler(t, st, &mockService{}, newFakeGoogle(t), nil)
	req := withUser(httptest.NewRequest(http.MethodPost, "/auth/google/calendar/consent-ack", nil), testUser)
	rec := httptest.NewRecorder()
	h.ConsentAck(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
}

// --- Connect ---

func TestConnect_NotConfigured(t *testing.T) {
	h := &CalendarIntegrationHandler{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	req := withUser(httptest.NewRequest(http.MethodGet, "/auth/google/calendar/connect", nil), testUser)
	rec := httptest.NewRecorder()
	h.Connect(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestConnect_MissingCalendarID(t *testing.T) {
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, newFakeGoogle(t), nil)
	req := withUser(httptest.NewRequest(http.MethodGet, "/auth/google/calendar/connect", nil), testUser)
	rec := httptest.NewRecorder()
	h.Connect(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestConnect_HappyPath_RedirectsAndSetsNonce(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, fg, nil)
	req := withUser(httptest.NewRequest(http.MethodGet,
		"/auth/google/calendar/connect?calendar_id=cal1&return_url=/calendars/cal1&tz=UTC", nil), testUser)
	rec := httptest.NewRecorder()
	h.Connect(rec, req)
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status: %d", rec.Code)
	}
	parsed, _ := url.Parse(rec.Header().Get("Location"))
	if parsed.Host == "" {
		t.Fatalf("redirect host empty: %q", rec.Header().Get("Location"))
	}
	stateParam := parsed.Query().Get("state")
	raw, err := base64.RawURLEncoding.DecodeString(stateParam)
	if err != nil {
		t.Fatalf("state decode: %v", err)
	}
	var s calendarOAuthState
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("state json: %v", err)
	}
	if s.CalendarID != "cal1" || s.ReturnURL != "/calendars/cal1" || s.TZ != "UTC" {
		t.Fatalf("state: %+v", s)
	}
	var nonce *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "calendar_oauth_nonce" {
			nonce = c
		}
	}
	if nonce == nil || nonce.Value != s.Nonce {
		t.Fatalf("nonce cookie missing or mismatched")
	}
}

func TestConnect_AbsoluteReturnURL_Coerced(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, fg, nil)
	req := withUser(httptest.NewRequest(http.MethodGet,
		"/auth/google/calendar/connect?calendar_id=cal1&return_url=https://evil/x", nil), testUser)
	rec := httptest.NewRecorder()
	h.Connect(rec, req)
	parsed, _ := url.Parse(rec.Header().Get("Location"))
	raw, _ := base64.RawURLEncoding.DecodeString(parsed.Query().Get("state"))
	var s calendarOAuthState
	_ = json.Unmarshal(raw, &s)
	if s.ReturnURL != "/calendars/cal1" {
		t.Fatalf("return_url not coerced: %q", s.ReturnURL)
	}
}

func TestConnect_RateLimited(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, fg, nil)
	// Tighten the limiter for this test.
	h.limiter = ratelimit.New(2, time.Hour)

	for i := 0; i < 2; i++ {
		req := withUser(httptest.NewRequest(http.MethodGet,
			"/auth/google/calendar/connect?calendar_id=cal1", nil), testUser)
		rec := httptest.NewRecorder()
		h.Connect(rec, req)
		if rec.Code != http.StatusTemporaryRedirect {
			t.Fatalf("call %d status: %d", i, rec.Code)
		}
	}
	req := withUser(httptest.NewRequest(http.MethodGet,
		"/auth/google/calendar/connect?calendar_id=cal1", nil), testUser)
	rec := httptest.NewRecorder()
	h.Connect(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
}

// --- Callback ---

// driveConnectThenCallback runs Connect, then issues Callback with the captured
// nonce cookie + an explicit state override (tests can pass nil to use the
// real one).
func driveCalendarCallback(t *testing.T, h *CalendarIntegrationHandler, code string, stateOverride []byte) *httptest.ResponseRecorder {
	t.Helper()
	connReq := withUser(httptest.NewRequest(http.MethodGet,
		"/auth/google/calendar/connect?calendar_id=cal1&return_url=/calendars/cal1&tz=UTC", nil), testUser)
	connRec := httptest.NewRecorder()
	h.Connect(connRec, connReq)
	if connRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("connect status: %d", connRec.Code)
	}
	parsed, _ := url.Parse(connRec.Header().Get("Location"))
	stateParam := parsed.Query().Get("state")
	if stateOverride != nil {
		stateParam = base64.RawURLEncoding.EncodeToString(stateOverride)
	}

	var nonce *http.Cookie
	for _, c := range connRec.Result().Cookies() {
		if c.Name == "calendar_oauth_nonce" {
			nonce = c
		}
	}
	cbReq := withUser(httptest.NewRequest(http.MethodGet,
		"/auth/google/calendar/callback?code="+code+"&state="+url.QueryEscape(stateParam), nil), testUser)
	if nonce != nil {
		cbReq.AddCookie(nonce)
	}
	cbRec := httptest.NewRecorder()
	h.Callback(cbRec, cbReq)
	return cbRec
}

func TestCallback_HappyPath_SetsSuggestionCookie(t *testing.T) {
	fg := newFakeGoogle(t)
	fbCalls := 0
	fb := func(_ context.Context, token string, _, _ time.Time, _ string) ([]availability.TimeRange, error) {
		fbCalls++
		if token != "AT" {
			t.Fatalf("freebusy token: %q", token)
		}
		return nil, nil
	}
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, fg, fb)

	rec := driveCalendarCallback(t, h, "abc", nil)
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	if fbCalls != 1 {
		t.Fatalf("freebusy calls: %d", fbCalls)
	}
	if loc := rec.Header().Get("Location"); loc != "https://app.test/calendars/cal1" {
		t.Fatalf("redirect: %q", loc)
	}
	var suggestion *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == suggestionCookieName("cal1") {
			suggestion = c
		}
	}
	if suggestion == nil || suggestion.Value == "" {
		t.Fatalf("suggestion cookie not set")
	}
}

func TestCallback_BadBase64State(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, fg, nil)
	req := withUser(httptest.NewRequest(http.MethodGet,
		"/auth/google/calendar/callback?code=x&state=!!not-base64!!", nil), testUser)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestCallback_BadJSONState(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, fg, nil)
	bad := base64.RawURLEncoding.EncodeToString([]byte("{not json"))
	req := withUser(httptest.NewRequest(http.MethodGet,
		"/auth/google/calendar/callback?code=x&state="+bad, nil), testUser)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestCallback_NonceMismatch(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, fg, nil)
	state := calendarOAuthState{Nonce: "expected", CalendarID: "cal1"}
	raw, _ := json.Marshal(state)
	rec := driveCalendarCallback(t, h, "abc", raw)
	// Connect set its own nonce cookie; we override the state with a different
	// nonce — should 400.
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestCallback_MissingCode(t *testing.T) {
	fg := newFakeGoogle(t)
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, fg, nil)
	rec := driveCalendarCallback(t, h, "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestCallback_TokenExchangeFails(t *testing.T) {
	fg := newFakeGoogle(t)
	fg.tokenHandler = func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, fg, nil)
	rec := driveCalendarCallback(t, h, "abc", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestCallback_FreeBusyFails(t *testing.T) {
	fg := newFakeGoogle(t)
	fb := func(_ context.Context, _ string, _, _ time.Time, _ string) ([]availability.TimeRange, error) {
		return nil, errors.New("freebusy boom")
	}
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, fg, fb)
	rec := driveCalendarCallback(t, h, "abc", nil)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status: %d", rec.Code)
	}
}

// --- GetSuggestions ---

func TestGetSuggestions_Unauthenticated(t *testing.T) {
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, newFakeGoogle(t), nil)
	req := httptest.NewRequest(http.MethodGet, "/calendars/cal1/import/suggestions", nil)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.GetSuggestions(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestGetSuggestions_CalendarNotFound(t *testing.T) {
	cal := &mockService{getCalendarOnly: func(_ context.Context, _ string) (model.Calendar, error) {
		return model.Calendar{}, store.ErrNotFound
	}}
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, cal, newFakeGoogle(t), nil)
	req := withUser(httptest.NewRequest(http.MethodGet, "/calendars/cal1/import/suggestions", nil), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.GetSuggestions(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestGetSuggestions_NoCookie(t *testing.T) {
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, newFakeGoogle(t), nil)
	req := withUser(httptest.NewRequest(http.MethodGet, "/calendars/cal1/import/suggestions", nil), testUser)
	req.SetPathValue("id", "cal1")
	rec := httptest.NewRecorder()
	h.GetSuggestions(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestGetSuggestions_HappyPath(t *testing.T) {
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, newFakeGoogle(t), nil)
	classified := availability.Classified{Free: []string{"2026-05-01"}, Busy: []string{"2026-05-02"}}
	payload, _ := json.Marshal(classified)
	cookieVal := base64.RawURLEncoding.EncodeToString(payload)

	req := withUser(httptest.NewRequest(http.MethodGet, "/calendars/cal1/import/suggestions", nil), testUser)
	req.SetPathValue("id", "cal1")
	req.AddCookie(&http.Cookie{Name: suggestionCookieName("cal1"), Value: cookieVal})
	rec := httptest.NewRecorder()
	h.GetSuggestions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string][]string
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got["free"]) != 1 || got["free"][0] != "2026-05-01" {
		t.Fatalf("free: %+v", got["free"])
	}
	// Cookie cleared.
	var cleared *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == suggestionCookieName("cal1") {
			cleared = c
		}
	}
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("cookie not cleared: %+v", cleared)
	}
}

func TestGetSuggestions_MalformedCookie(t *testing.T) {
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, newFakeGoogle(t), nil)
	req := withUser(httptest.NewRequest(http.MethodGet, "/calendars/cal1/import/suggestions", nil), testUser)
	req.SetPathValue("id", "cal1")
	req.AddCookie(&http.Cookie{Name: suggestionCookieName("cal1"), Value: "!!not-base64!!"})
	rec := httptest.NewRecorder()
	h.GetSuggestions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestGetSuggestions_MalformedJSON(t *testing.T) {
	h := newCalIntegrationHandler(t, &fakeCalIntegrationStore{}, &mockService{}, newFakeGoogle(t), nil)
	bad := base64.RawURLEncoding.EncodeToString([]byte("{not json"))
	req := withUser(httptest.NewRequest(http.MethodGet, "/calendars/cal1/import/suggestions", nil), testUser)
	req.SetPathValue("id", "cal1")
	req.AddCookie(&http.Cookie{Name: suggestionCookieName("cal1"), Value: bad})
	rec := httptest.NewRecorder()
	h.GetSuggestions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}
