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
	CreateSession(ctx context.Context, userID string) (string, error)
	GetSession(ctx context.Context, token string) (*model.Session, error)
	DeleteSession(ctx context.Context, token string) error
	CreateMagicLinkToken(ctx context.Context, email, returnURL string) (string, error)
	VerifyMagicLinkToken(ctx context.Context, plainToken string) (email, returnURL string, err error)
}

// EmailSender sends transactional emails.
type EmailSender interface {
	SendMagicLink(to, link string) error
}

type AuthHandler struct {
	store        AuthStore
	email        EmailSender
	oauth        *oauth2.Config
	frontendURL  string
	cookieDomain string
	secureCookie bool
	logger       *slog.Logger
}

func NewAuth(s AuthStore, emailSender EmailSender, cfg *config.Config, logger *slog.Logger) *AuthHandler {
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

	return &AuthHandler{
		store:        s,
		email:        emailSender,
		oauth:        oauthCfg,
		frontendURL:  cfg.FrontendURL,
		cookieDomain: cfg.CookieDomain,
		secureCookie: cfg.Env != "dev",
		logger:       logger,
	}
}

// oauthState is encoded as JSON in the OAuth state parameter.
type oauthState struct {
	Nonce     string `json:"nonce"`
	ReturnURL string `json:"return_url,omitempty"`
}

// GoogleLogin redirects to the Google OAuth consent screen.
func (h *AuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if h.oauth == nil {
		respondError(w, http.StatusServiceUnavailable, "Google OAuth not configured")
		return
	}

	nonce := generateNonce()
	returnURL := r.URL.Query().Get("return_url")
	if !isRelativePath(returnURL) {
		returnURL = "/"
	}

	state, err := json.Marshal(oauthState{Nonce: nonce, ReturnURL: returnURL})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to encode state")
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
		respondError(w, http.StatusServiceUnavailable, "Google OAuth not configured")
		return
	}

	// Verify state nonce
	var state oauthState
	if err := json.Unmarshal([]byte(r.URL.Query().Get("state")), &state); err != nil {
		respondError(w, http.StatusBadRequest, "invalid state")
		return
	}

	nonceCookie, err := r.Cookie("oauth_nonce")
	if err != nil || nonceCookie.Value != state.Nonce {
		respondError(w, http.StatusBadRequest, "invalid state nonce")
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
		respondError(w, http.StatusBadRequest, "missing code")
		return
	}

	token, err := h.oauth.Exchange(r.Context(), code)
	if err != nil {
		h.logger.Error("oauth exchange failed", "error", err)
		respondError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	// Fetch user info from Google
	client := h.oauth.Client(r.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		h.logger.Error("failed to fetch user info", "error", err)
		respondError(w, http.StatusInternalServerError, "authentication failed")
		return
	}
	defer resp.Body.Close()

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		h.logger.Error("failed to decode user info", "error", err)
		respondError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	// Create or find user
	user, err := h.findOrCreateUser(r.Context(), info)
	if err != nil {
		h.logger.Error("failed to find or create user", "error", err)
		respondError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	// Create session
	sessionToken, err := h.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		respondError(w, http.StatusInternalServerError, "authentication failed")
		return
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
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	sess, err := h.store.GetSession(r.Context(), cookie.Value)
	if err != nil || sess == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	user, err := h.store.GetUser(r.Context(), sess.UserID)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// Logout deletes the session and clears the cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		_ = h.store.DeleteSession(r.Context(), cookie.Value)
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

// magicLinkRequest is the JSON body for POST /auth/magic-link.
type magicLinkRequest struct {
	Email     string `json:"email"`
	ReturnURL string `json:"return_url"`
}

// SendMagicLink generates a magic link token and emails it to the user.
func (h *AuthHandler) SendMagicLink(w http.ResponseWriter, r *http.Request) {
	if h.email == nil {
		respondError(w, http.StatusServiceUnavailable, "magic link not configured")
		return
	}

	var req magicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !isValidEmail(req.Email) {
		respondError(w, http.StatusBadRequest, "invalid email address")
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
			h.logger.Error("failed to send magic link email", "error", err, "email", req.Email)
		}
	}

	// Always return success — don't leak whether the email was sent
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// MagicLinkCallback verifies the magic link token, creates/finds the user,
// creates a session, and redirects to the return URL.
func (h *AuthHandler) MagicLinkCallback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		respondError(w, http.StatusBadRequest, "missing token")
		return
	}

	email, returnURL, err := h.store.VerifyMagicLinkToken(r.Context(), token)
	if err != nil {
		h.logger.Error("magic link verification failed", "error", err)
		respondError(w, http.StatusBadRequest, "invalid or expired link")
		return
	}

	// Create or find user with provider "email"
	user, err := h.findOrCreateEmailUser(r.Context(), email)
	if err != nil {
		h.logger.Error("failed to find or create user", "error", err)
		respondError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	sessionToken, err := h.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		respondError(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	h.setSessionCookie(w, sessionToken)

	if !isRelativePath(returnURL) {
		returnURL = "/"
	}
	http.Redirect(w, r, h.frontendURL+returnURL, http.StatusTemporaryRedirect)
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

