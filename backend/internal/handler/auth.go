package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/freewhen/backend/internal/config"
	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/store"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// AuthStore defines the data operations the auth handler needs.
type AuthStore interface {
	CreateUser(ctx context.Context, email, name, provider, providerID string) (model.User, error)
	GetUser(ctx context.Context, id string) (model.User, error)
	FindUserByEmail(ctx context.Context, email string) (model.User, error)
	FindUserByProviderID(ctx context.Context, provider, providerID string) (model.User, error)
	UpdateUserName(ctx context.Context, userID, name string) (model.User, error)
	CreateSession(ctx context.Context, userID string) (string, error)
	GetSession(ctx context.Context, token string) (*model.Session, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteSessionsForUser(ctx context.Context, userID string) error
	PruneSessionsForUser(ctx context.Context, userID string, keep int) error
	CreateMagicLinkToken(ctx context.Context, email, returnURL string) (string, error)
	VerifyMagicLinkToken(ctx context.Context, plainToken string) (email, returnURL string, err error)
}

// EmailSender sends transactional emails.
type EmailSender interface {
	SendMagicLink(to, link string) error
}

type AuthHandler struct {
	store             AuthStore
	email             EmailSender
	oauth             *oauth2.Config
	userInfoURL       string // overridable for tests
	frontendURL       string
	cookieDomain      string
	secureCookie      bool
	logger            *slog.Logger
	magicLimiter      *magicLinkLimiter
	trustedProxyDepth int
}

const defaultGoogleUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

// maxConcurrentSessionsPerUser caps active sessions per user (auth audit F11).
// Bounds attacker accumulation of parallel sessions within the F2 absolute
// expiry window, while staying loose enough that a real user on phone +
// laptop + work laptop + tablet doesn't get bumped on a fresh login.
const maxConcurrentSessionsPerUser = 10

// AuthOption configures optional AuthHandler fields. Production callers pass
// none — these exist so tests can point the OAuth/userinfo URLs at an
// httptest.Server without also faking out the network.
type AuthOption func(*AuthHandler)

// WithOAuthEndpoint overrides the OAuth token/auth endpoint (default: google.Endpoint).
func WithOAuthEndpoint(ep oauth2.Endpoint) AuthOption {
	return func(h *AuthHandler) {
		if h.oauth != nil {
			h.oauth.Endpoint = ep
		}
	}
}

// WithUserInfoURL overrides the userinfo URL (default: Google's v3 userinfo).
func WithUserInfoURL(u string) AuthOption {
	return func(h *AuthHandler) { h.userInfoURL = u }
}

func NewAuth(s AuthStore, emailSender EmailSender, cfg *config.Config, logger *slog.Logger, opts ...AuthOption) *AuthHandler {
	var oauthCfg *oauth2.Config
	if cfg.GoogleClientID != "" {
		oauthCfg = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		}
	}

	h := &AuthHandler{
		store:             s,
		email:             emailSender,
		oauth:             oauthCfg,
		userInfoURL:       defaultGoogleUserInfoURL,
		frontendURL:       cfg.FrontendURL,
		cookieDomain:      cfg.CookieDomain,
		secureCookie:      cfg.SecureCookies,
		logger:            logger,
		magicLimiter:      newMagicLinkLimiter(),
		trustedProxyDepth: cfg.TrustedProxyDepth,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// oauthState is encoded as JSON in the OAuth state parameter.
type oauthState struct {
	Nonce     string `json:"nonce"`
	ReturnURL string `json:"return_url,omitempty"`
}

// GoogleLogin redirects to the Google OAuth consent screen.
func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if h.oauth == nil {
		respondError(w, http.StatusServiceUnavailable, "service_unavailable", "Google OAuth not configured")
		return
	}

	nonce := generateNonce()
	returnURL := r.URL.Query().Get("return_url")
	if !isRelativePath(returnURL) {
		returnURL = "/"
	}

	state, err := json.Marshal(oauthState{Nonce: nonce, ReturnURL: returnURL})
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to encode state")
		return
	}

	// Store nonce in a short-lived cookie for CSRF verification
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_nonce",
		Value:    nonce,
		Path:     "/auth/google/callback",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Domain:   h.cookieDomain,
	})

	url := h.oauth.AuthCodeURL(string(state))
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// googleUserInfo is the response from Google's userinfo endpoint.
type googleUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// GoogleCallback handles the Google OAuth callback.
func (h *AuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h.oauth == nil {
		respondError(w, http.StatusServiceUnavailable, "service_unavailable", "Google OAuth not configured")
		return
	}

	// Verify state nonce
	var state oauthState
	if err := json.Unmarshal([]byte(r.URL.Query().Get("state")), &state); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid state")
		return
	}

	nonceCookie, err := r.Cookie("oauth_nonce")
	if err != nil || nonceCookie.Value != state.Nonce {
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid state nonce")
		return
	}

	// Clear the nonce cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_nonce",
		Value:    "",
		Path:     "/auth/google/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Domain:   h.cookieDomain,
	})

	// Exchange code for token
	code := r.URL.Query().Get("code")
	if code == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "missing code")
		return
	}

	token, err := h.oauth.Exchange(r.Context(), code)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "authentication failed")
		return
	}

	// Fetch user info from Google
	client := h.oauth.Client(r.Context(), token)
	resp, err := client.Get(h.userInfoURL)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "authentication failed")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		respondInternalError(r, w, err, "internal_error", "authentication failed")
		return
	}
	info.Email = normalizeEmail(info.Email)

	// Create or find user
	user, err := h.findOrCreateUser(r.Context(), info)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "authentication failed")
		return
	}

	// Create session
	sessionToken, err := h.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "authentication failed")
		return
	}

	if err := h.store.PruneSessionsForUser(r.Context(), user.ID, maxConcurrentSessionsPerUser); err != nil {
		h.logger.Error("failed to prune sessions on google login", "error", err)
	}

	h.setSessionCookie(w, sessionToken)

	// Redirect to frontend
	returnURL := state.ReturnURL
	if !isRelativePath(returnURL) {
		returnURL = "/"
	}
	http.Redirect(w, r, h.frontendURL+returnURL, http.StatusTemporaryRedirect)
}

// Me returns the currently authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	sess, err := h.store.GetSession(r.Context(), cookie.Value)
	if err != nil || sess == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	user, err := h.store.GetUser(r.Context(), sess.UserID)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// updateMeRequest is the JSON body for PATCH /auth/me.
type updateMeRequest struct {
	Name *string `json:"name"`
}

// UpdateMe updates the authenticated user's profile fields (currently: name).
func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	var req updateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	if req.Name == nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "no fields to update")
		return
	}

	name := strings.TrimSpace(*req.Name)
	if name == "" {
		respondError(w, http.StatusBadRequest, "invalid_name", "name cannot be empty")
		return
	}
	if len([]rune(name)) > limits.MaxUserNameLength {
		respondError(w, http.StatusBadRequest, "invalid_name", "name is too long")
		return
	}

	updated, err := h.store.UpdateUserName(r.Context(), user.ID, name)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to update")
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

// Logout deletes the session and clears the cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		if delErr := h.store.DeleteSession(r.Context(), cookie.Value); delErr != nil {
			h.logger.Error("failed to delete session on logout", "error", delErr)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Domain:   h.cookieDomain,
	})

	respondJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// LogoutAll deletes every session for the authenticated user and immediately
// re-issues a fresh session for the current device, replacing the cookie.
//
// Audit F12 — gives a user with a suspected stolen token a self-service
// revocation lever. PruneSessionsForUser already bounds attacker accumulation
// to 10 parallel sessions, and the F2 absolute cap bounds them to 90 days,
// but neither is user-triggerable. After this call returns, every session
// row except the freshly-minted one is gone.
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	if err := h.store.DeleteSessionsForUser(r.Context(), user.ID); err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to revoke sessions")
		return
	}

	// Re-issue a fresh session for the current device so the user stays
	// logged in here. Order matters: the delete above wiped the cookie's
	// session row too, so this row is the only one that exists for the user
	// once we return.
	sessionToken, err := h.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "failed to revoke sessions")
		return
	}
	h.setSessionCookie(w, sessionToken)

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// magicLinkRequest is the JSON body for POST /auth/magic-link.
type magicLinkRequest struct {
	Email     string `json:"email"`
	ReturnURL string `json:"return_url"`
}

// SendMagicLink generates a magic link token and emails it to the user.
//
// F6 invariant: this handler never branches on whether `req.Email` belongs to
// a known user. `findOrCreateEmailUser` runs at callback time, not here. That
// keeps the response time uniform between known and unknown emails so an
// attacker can't enumerate accounts via timing. If you ever add a "skip if
// user doesn't exist" optimization, you reopen the timing channel and must
// add a constant-time floor to compensate.
func (h *AuthHandler) SendMagicLink(w http.ResponseWriter, r *http.Request) {
	if h.email == nil {
		respondError(w, http.StatusServiceUnavailable, "service_unavailable", "magic link not configured")
		return
	}

	var req magicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}

	req.Email = normalizeEmail(req.Email)
	if !isValidEmail(req.Email) {
		respondError(w, http.StatusBadRequest, "invalid_email", "invalid email address")
		return
	}

	if !h.magicLimiter.allow(req.Email, remoteIP(r, h.trustedProxyDepth)) {
		w.Header().Set("Retry-After", "900")
		respondError(w, http.StatusTooManyRequests, "rate_limited", "too many requests, try again shortly")
		return
	}

	returnURL := req.ReturnURL
	if !isRelativePath(returnURL) {
		returnURL = "/"
	}

	token, err := h.store.CreateMagicLinkToken(r.Context(), req.Email, returnURL)
	if err != nil {
		h.logger.Error("failed to create magic link token", "error", err)
		// Always return 200 — don't leak whether the email exists or not
	}

	if token != "" {
		link := fmt.Sprintf("%s/auth/magic-link/callback?token=%s", h.frontendURL, token)
		if err := h.email.SendMagicLink(req.Email, link); err != nil {
			h.logger.Error("failed to send magic link email", "error", err)
		}
	}

	// Always return success — don't leak whether the email was sent
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// magicLinkVerifyRequest is the JSON body for POST /auth/magic-link/verify.
type magicLinkVerifyRequest struct {
	Token string `json:"token"`
}

// magicLinkVerifyResponse is returned to the frontend interstitial after a
// successful exchange. The frontend then navigates to ReturnURL.
type magicLinkVerifyResponse struct {
	ReturnURL string `json:"return_url"`
}

// MagicLinkVerify consumes a magic-link token, creates/finds the user, sets
// the session cookie, and returns the validated return URL.
//
// F4 mitigation: the email link points at the frontend (`FRONTEND_URL/auth/
// magic-link/callback?token=...`), which renders an interstitial page rather
// than auto-redirecting. Token consumption only happens on this POST, which
// link prefetchers (Outlook SafeLinks, Slack unfurl, AV scanners) don't
// trigger because they neither execute JS nor follow user-initiated POSTs.
// Do not add a GET variant of this endpoint — that re-opens the prefetcher
// hole and is the bug this PR closed.
func (h *AuthHandler) MagicLinkVerify(w http.ResponseWriter, r *http.Request) {
	var req magicLinkVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "invalid request body")
		return
	}
	if req.Token == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "missing token")
		return
	}

	email, returnURL, err := h.store.VerifyMagicLinkToken(r.Context(), req.Token)
	if err != nil {
		h.logger.Error("magic link verification failed", "error", err)
		respondError(w, http.StatusBadRequest, "invalid_request", "invalid or expired link")
		return
	}
	email = normalizeEmail(email)

	user, err := h.findOrCreateEmailUser(r.Context(), email)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "authentication failed")
		return
	}

	sessionToken, err := h.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "authentication failed")
		return
	}

	if err := h.store.PruneSessionsForUser(r.Context(), user.ID, maxConcurrentSessionsPerUser); err != nil {
		h.logger.Error("failed to prune sessions on magic link login", "error", err)
	}

	h.setSessionCookie(w, sessionToken)

	if !isRelativePath(returnURL) {
		returnURL = "/"
	}
	respondJSON(w, http.StatusOK, magicLinkVerifyResponse{ReturnURL: returnURL})
}

func (h *AuthHandler) findOrCreateEmailUser(ctx context.Context, emailAddr string) (model.User, error) {
	user, err := h.store.FindUserByEmail(ctx, emailAddr)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return model.User{}, fmt.Errorf("find user by email: %w", err)
	}

	return h.store.CreateUser(ctx, emailAddr, "", "email", "")
}

func (h *AuthHandler) findOrCreateUser(ctx context.Context, info googleUserInfo) (model.User, error) {
	// Try to find by provider ID first
	user, err := h.store.FindUserByProviderID(ctx, "google", info.Sub)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return model.User{}, fmt.Errorf("find user by provider ID: %w", err)
	}

	// Try to find by email (user may have signed up via magic link previously)
	user, err = h.store.FindUserByEmail(ctx, info.Email)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return model.User{}, fmt.Errorf("find user by email: %w", err)
	}

	// Create new user
	return h.store.CreateUser(ctx, info.Email, info.Name, "google", info.Sub)
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Domain:   h.cookieDomain,
	})
}

func generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// isRelativePath validates that the URL is a relative path (starts with /).
// Prevents open redirect attacks.
func isRelativePath(s string) bool {
	if s == "" {
		return false
	}
	return strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//")
}

// normalizeEmail lowercases and trims surrounding whitespace, producing the
// canonical form used everywhere downstream.
//
// Audit F15: every email-shaped boundary input must be normalized before it
// reaches the rate limiter, the magic-link store, or the users table.
// Otherwise `User@x.com` and `user@x.com` get separate rate-limit buckets,
// separate concurrent magic-link tokens, and (worst case) separate user rows
// — an account split. Migration 023 lowercased every existing row, so all
// stored values are already canonical; this keeps new writes consistent.
func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// isValidEmail does a basic check that the string looks like an email address.
func isValidEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at < 1 || at >= len(email)-1 {
		return false
	}
	return strings.Contains(email[at+1:], ".")
}
