package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/freewhen/backend/internal/availability"
	"github.com/freewhen/backend/internal/config"
	googlecal "github.com/freewhen/backend/internal/google"
	"github.com/freewhen/backend/internal/ratelimit"
	"github.com/freewhen/backend/internal/store"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// CalendarIntegrationStore is the subset of the store the integration
// handler needs. Kept narrow so tests can stub it without pulling in the
// whole Store type.
type CalendarIntegrationStore interface {
	SetCalendarImportConsentAck(ctx context.Context, userID string) error
}

// CalendarService interface is reused from handler.go.

// CalendarIntegrationHandler wires together the incremental-OAuth flow for
// importing a user's Google Calendar free/busy data. No tokens are
// persisted — the access token is used once, inside the callback, and then
// discarded.
type CalendarIntegrationHandler struct {
	store        CalendarIntegrationStore
	cal          CalendarService
	oauth        *oauth2.Config
	frontendURL  string
	cookieDomain string
	secureCookie bool
	logger       *slog.Logger
	limiter      *ratelimit.Bucket
	freeBusy     FreeBusyFetcher
}

// FreeBusyFetcher is the seam for injecting the Google FreeBusy call. The
// production implementation calls Google; tests supply a stub.
type FreeBusyFetcher func(ctx context.Context, accessToken string, start, end time.Time, tz string) ([]availability.TimeRange, error)

// CalendarIntegrationOption configures optional fields on the handler. Used
// by tests to point the OAuth endpoint at an httptest.Server and to swap in
// a stub FreeBusyFetcher.
type CalendarIntegrationOption func(*CalendarIntegrationHandler)

// WithCalendarOAuthEndpoint overrides the OAuth token/auth endpoint.
func WithCalendarOAuthEndpoint(ep oauth2.Endpoint) CalendarIntegrationOption {
	return func(h *CalendarIntegrationHandler) {
		if h.oauth != nil {
			h.oauth.Endpoint = ep
		}
	}
}

// WithFreeBusyFetcher overrides the FreeBusy fetcher seam.
func WithFreeBusyFetcher(fb FreeBusyFetcher) CalendarIntegrationOption {
	return func(h *CalendarIntegrationHandler) { h.freeBusy = fb }
}

// NewCalendarIntegration wires the handler using the existing Google client
// credentials with the calendar-specific redirect URL and scope.
func NewCalendarIntegration(s CalendarIntegrationStore, cal CalendarService, cfg *config.Config, logger *slog.Logger, opts ...CalendarIntegrationOption) *CalendarIntegrationHandler {
	var oauthCfg *oauth2.Config
	if cfg.GoogleClientID != "" && cfg.GoogleCalendarRedirectURL != "" {
		oauthCfg = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleCalendarRedirectURL,
			Scopes:       []string{"https://www.googleapis.com/auth/calendar.freebusy"},
			Endpoint:     google.Endpoint,
		}
	}

	h := &CalendarIntegrationHandler{
		store:        s,
		cal:          cal,
		oauth:        oauthCfg,
		frontendURL:  cfg.FrontendURL,
		cookieDomain: cfg.CookieDomain,
		secureCookie: cfg.SecureCookies,
		logger:       logger,
		limiter:      ratelimit.New(10, time.Hour),
		freeBusy: func(ctx context.Context, token string, start, end time.Time, tz string) ([]availability.TimeRange, error) {
			return googlecal.FreeBusy(ctx, nil, token, start, end, tz)
		},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// ConsentAck records that the user has seen the privacy interstitial.
// Idempotent — always writes now(). Called by the interstitial's "Continue"
// action right before the OAuth redirect.
func (h *CalendarIntegrationHandler) ConsentAck(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	if err := h.store.SetCalendarImportConsentAck(r.Context(), user.ID); err != nil {
		h.logger.Error("set consent ack failed", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to record ack")
		return
	}
	h.logger.Info("calendar_consent_given", "user_id", user.ID, "timestamp", time.Now().UTC())
	w.WriteHeader(http.StatusNoContent)
}

type calendarOAuthState struct {
	Nonce      string `json:"nonce"`
	CalendarID string `json:"calendar_id"`
	ReturnURL  string `json:"return_url,omitempty"`
	TZ         string `json:"tz,omitempty"`
}

// Connect kicks off the incremental-OAuth round-trip. The caller is expected
// to have already ack'd consent (the frontend gates this). We encode the
// calendar ID / return URL / timezone into state so the callback has
// everything it needs to compute suggestions and redirect.
func (h *CalendarIntegrationHandler) Connect(w http.ResponseWriter, r *http.Request) {
	if h.oauth == nil {
		respondError(w, http.StatusServiceUnavailable, "service_unavailable", "calendar OAuth not configured")
		return
	}
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	if !h.limiter.Allow(user.ID) {
		respondError(w, http.StatusTooManyRequests, "rate_limited", "too many import attempts, try again later")
		return
	}

	calendarID := r.URL.Query().Get("calendar_id")
	if calendarID == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "calendar_id required")
		return
	}

	returnURL := r.URL.Query().Get("return_url")
	if !isRelativePath(returnURL) {
		returnURL = "/calendars/" + calendarID
	}

	tz := r.URL.Query().Get("tz")
	if tz == "" {
		tz = "UTC"
	}

	state := calendarOAuthState{
		Nonce:      generateNonce(),
		CalendarID: calendarID,
		ReturnURL:  returnURL,
		TZ:         tz,
	}
	raw, err := json.Marshal(state)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to encode state")
		return
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)

	http.SetCookie(w, &http.Cookie{
		Name:     "calendar_oauth_nonce",
		Value:    state.Nonce,
		Path:     "/auth/google/calendar/callback",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Domain:   h.cookieDomain,
	})

	url := h.oauth.AuthCodeURL(encoded,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Callback completes the round-trip: exchange code → FreeBusy → compute
// per-day vector → store it in a short-lived single-use cookie → redirect
// back to the calendar page.
func (h *CalendarIntegrationHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if h.oauth == nil {
		respondError(w, http.StatusServiceUnavailable, "service_unavailable", "calendar OAuth not configured")
		return
	}
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	encoded := r.URL.Query().Get("state")
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid state")
		return
	}
	var state calendarOAuthState
	if err := json.Unmarshal(raw, &state); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid state")
		return
	}

	nonceCookie, err := r.Cookie("calendar_oauth_nonce")
	if err != nil || nonceCookie.Value != state.Nonce {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid state nonce")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "calendar_oauth_nonce",
		Value:    "",
		Path:     "/auth/google/calendar/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Domain:   h.cookieDomain,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "missing code")
		return
	}

	token, err := h.oauth.Exchange(r.Context(), code)
	if err != nil {
		h.logger.Error("calendar oauth exchange failed", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "authentication failed")
		return
	}

	// Decide the import window. The plan references calendar.start_date /
	// end_date but those don't exist — availability lives inside the
	// limits.DateWithinRange window. We use "today → today+90 days" as a
	// reasonable default for "upcoming free time".
	tz, err := time.LoadLocation(state.TZ)
	if err != nil {
		tz = time.UTC
	}
	now := time.Now().In(tz)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tz)
	end := start.AddDate(0, 0, 90)

	busy, err := h.freeBusy(r.Context(), token.AccessToken, start, end, state.TZ)
	if err != nil {
		h.logger.Error("freebusy fetch failed", "error", err, "user_id", user.ID)
		respondError(w, http.StatusBadGateway, "freebusy_failed", "failed to read free/busy")
		return
	}

	classified := availability.ComputeFreeDays(busy, tz, start, end, time.Now(), availability.FreeDayOptions{})

	suggestPayload, err := json.Marshal(classified)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to encode suggestions")
		return
	}
	cookieVal := base64.RawURLEncoding.EncodeToString(suggestPayload)

	http.SetCookie(w, &http.Cookie{
		Name:     suggestionCookieName(state.CalendarID),
		Value:    cookieVal,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Domain:   h.cookieDomain,
	})

	h.logger.Info("calendar_import", "user_id", user.ID, "calendar_id", state.CalendarID,
		"free_count", len(classified.Free), "busy_count", len(classified.Busy))

	returnURL := state.ReturnURL
	if !isRelativePath(returnURL) {
		returnURL = "/calendars/" + state.CalendarID
	}
	http.Redirect(w, r, h.frontendURL+returnURL, http.StatusTemporaryRedirect)
}

// GetSuggestions returns the suggestion vector stashed in the import cookie
// and clears the cookie in the same response — single-use by design.
func (h *CalendarIntegrationHandler) GetSuggestions(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	calendarID := r.PathValue("id")
	// Validate the caller can read this calendar so we don't surface
	// someone else's cookie content (cookies scope by name only).
	if _, err := h.cal.GetCalendarOnly(r.Context(), calendarID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "calendar not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "failed to load calendar")
		return
	}

	name := suggestionCookieName(calendarID)
	cookie, err := r.Cookie(name)
	if err != nil {
		respondError(w, http.StatusNotFound, "no_suggestions", "no pending import")
		return
	}

	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_cookie", "malformed suggestion cookie")
		return
	}

	var classified availability.Classified
	if err := json.Unmarshal(raw, &classified); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_cookie", "malformed suggestion cookie")
		return
	}

	// Clear the cookie (single-use).
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Domain:   h.cookieDomain,
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"free": emptyOrSlice(classified.Free),
		"busy": emptyOrSlice(classified.Busy),
	})
}

func emptyOrSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// suggestionCookieName is the per-calendar cookie name. Kept short — cookies
// share a header budget with the session cookie.
func suggestionCookieName(calendarID string) string {
	// Sanitise to safe cookie-name chars. Calendar IDs are short alnum so
	// this is belt-and-braces against future ID scheme changes.
	safe := make([]byte, 0, len(calendarID))
	for i := 0; i < len(calendarID); i++ {
		c := calendarID[i]
		switch {
		case c >= '0' && c <= '9', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '_', c == '-':
			safe = append(safe, c)
		}
	}
	return "fw_imp_" + strings.ToLower(string(safe))
}
