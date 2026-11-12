package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/freewhen/backend/internal/model"
)

// TestLoginHandler mints a session for a given email without going through
// OAuth or magic-link. Wired only when config.E2ETestMode is true (which is
// itself refused when SECURE_COOKIES=true). Used by the Playwright suite to
// fast-path through auth in non-auth-focused specs.
type TestLoginHandler struct {
	store        AuthStore
	cookieDomain string
	secureCookie bool
	logger       *slog.Logger
}

func NewTestLogin(s AuthStore, cookieDomain string, secureCookie bool, logger *slog.Logger) *TestLoginHandler {
	return &TestLoginHandler{
		store:        s,
		cookieDomain: cookieDomain,
		secureCookie: secureCookie,
		logger:       logger,
	}
}

type testLoginRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Login finds-or-creates the user with the given email and sets a session
// cookie. Always 200 on success, regardless of whether the user existed.
func (h *TestLoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req testLoginRequest
	if !decodeJSONMessage(w, r, &req, "invalid request body") {
		return
	}
	req.Email = normalizeEmail(req.Email)
	if !isValidEmail(req.Email) {
		respondError(w, http.StatusBadRequest, "invalid_email", "invalid email address")
		return
	}

	user, err := h.findOrCreate(r.Context(), req.Email, req.Name)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "test-login failed")
		return
	}

	token, err := h.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		respondInternalError(r, w, err, "internal_error", "test-login failed")
		return
	}

	if err := h.store.PruneSessionsForUser(r.Context(), user.ID, maxConcurrentSessionsPerUser); err != nil {
		h.logger.Error("failed to prune sessions on test login", "error", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		Domain:   h.cookieDomain,
	})

	respondJSON(w, http.StatusOK, user)
}

func (h *TestLoginHandler) findOrCreate(ctx context.Context, email, name string) (model.User, error) {
	user, err := h.store.FindUserByEmail(ctx, email)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		return model.User{}, err
	}
	return h.store.CreateUser(ctx, email, name, "e2e", "")
}
