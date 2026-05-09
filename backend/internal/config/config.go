package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL               string
	Port                      string
	LogLevel                  slog.Level
	SecureCookies             bool
	CORSOrigin                string
	GoogleClientID            string
	GoogleClientSecret        string
	GoogleRedirectURL         string
	GoogleCalendarRedirectURL string
	FrontendURL               string
	CookieDomain              string
	ResendAPIKey              string
	MagicLinkFromEmail        string
	// AuthEmailDisabled is set when AUTH_EMAIL_DISABLED=true; callers should
	// skip magic-link email wiring rather than inferring from empty keys.
	AuthEmailDisabled bool
	// CalendarAccessSecret is the HMAC key used to sign per-calendar access
	// tokens (password-protected calendars). Must be at least 32 bytes.
	CalendarAccessSecret string
	// NotificationsHMACSecret signs one-click unsubscribe tokens. Must be at
	// least 32 bytes when notifications are enabled. When unset, the
	// notifications subsystem is disabled (no finalised/nudge emails sent and
	// /unsubscribe returns 404).
	NotificationsHMACSecret string
	// PostHogKey is the project API key for PostHog. When unset, flag
	// evaluation no-ops and analytics events are dropped server-side.
	PostHogKey string
	// PostHogHost overrides the PostHog API host (e.g. https://eu.i.posthog.com).
	PostHogHost string
	// PostHogPersonalKey enables local polling of flag definitions for
	// in-process evaluation. Optional.
	PostHogPersonalKey string
	// E2ETestMode enables a /test/login endpoint that mints sessions for an
	// arbitrary email — used by the Playwright suite. Hard-refused when
	// SECURE_COOKIES=true so it can never be enabled in a prod-shaped env.
	E2ETestMode bool
	// TrustedProxyDepth is the number of trailing X-Forwarded-For entries that
	// belong to infrastructure we control (load balancers, ingress) and can
	// trust. The client IP is read at index len-1-depth, so a malicious client
	// supplying its own XFF header cannot poison rate-limit buckets — anything
	// it injects appears further left and is discarded. Default 0 matches
	// Cloud Run direct ingress, which appends only the actual client IP.
	// Increase to 1 if running behind a Google external HTTPS load balancer
	// (which appends an additional LB hop).
	TrustedProxyDepth int
}

func Load() (*Config, error) {
	_ = godotenv.Load(".env.local", ".env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:3000"
	}

	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		return nil, err
	}

	secureCookies, err := parseBool(os.Getenv("SECURE_COOKIES"), true)
	if err != nil {
		return nil, fmt.Errorf("SECURE_COOKIES: %w", err)
	}

	accessSecret := os.Getenv("CALENDAR_ACCESS_SECRET")
	if len(accessSecret) < 32 {
		return nil, fmt.Errorf("CALENDAR_ACCESS_SECRET environment variable must be at least 32 bytes")
	}

	notifSecret := os.Getenv("NOTIFICATIONS_HMAC_SECRET")
	if notifSecret != "" && len(notifSecret) < 32 {
		return nil, fmt.Errorf("NOTIFICATIONS_HMAC_SECRET must be at least 32 bytes when set (or unset to disable notifications)")
	}

	authEmailDisabled, err := parseBool(os.Getenv("AUTH_EMAIL_DISABLED"), false)
	if err != nil {
		return nil, fmt.Errorf("AUTH_EMAIL_DISABLED: %w", err)
	}

	resendKey := os.Getenv("RESEND_API_KEY")
	fromEmail := os.Getenv("MAGIC_LINK_FROM_EMAIL")
	frontendURL := os.Getenv("FRONTEND_URL")

	if err := validateAuthEmail(authEmailDisabled, resendKey, fromEmail, frontendURL); err != nil {
		return nil, err
	}

	googleID := os.Getenv("GOOGLE_CLIENT_ID")
	googleSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	googleRedirect := os.Getenv("GOOGLE_REDIRECT_URL")
	googleCalRedirect := os.Getenv("GOOGLE_CALENDAR_REDIRECT_URL")

	if err := validateGoogleOAuth(googleID, googleSecret, googleRedirect, googleCalRedirect); err != nil {
		return nil, err
	}

	cookieDomain := os.Getenv("COOKIE_DOMAIN")
	if secureCookies && cookieDomain == "" {
		return nil, fmt.Errorf("COOKIE_DOMAIN is required when SECURE_COOKIES=true (set SECURE_COOKIES=false for local HTTP dev)")
	}

	allowParentDomainCookie, err := parseBool(os.Getenv("ALLOW_PARENT_DOMAIN_COOKIE"), false)
	if err != nil {
		return nil, fmt.Errorf("ALLOW_PARENT_DOMAIN_COOKIE: %w", err)
	}
	// A leading-dot COOKIE_DOMAIN scopes the session cookie to every subdomain
	// of the parent domain. Anyone who can stand up a subdomain there can
	// read/set the session cookie. Force an explicit opt-in so this dangerous
	// shape is a deliberate decision, not a quiet default.
	if len(cookieDomain) > 0 && cookieDomain[0] == '.' && !allowParentDomainCookie {
		return nil, fmt.Errorf("COOKIE_DOMAIN=%q is a parent-domain cookie (leading dot); set ALLOW_PARENT_DOMAIN_COOKIE=true to confirm or scope COOKIE_DOMAIN to a single host", cookieDomain)
	}

	trustedProxyDepth, err := parseNonNegativeInt(os.Getenv("TRUSTED_PROXY_DEPTH"), 0)
	if err != nil {
		return nil, fmt.Errorf("TRUSTED_PROXY_DEPTH: %w", err)
	}

	e2eTestMode, err := parseBool(os.Getenv("E2E_TEST_MODE"), false)
	if err != nil {
		return nil, fmt.Errorf("E2E_TEST_MODE: %w", err)
	}
	if e2eTestMode && secureCookies {
		return nil, fmt.Errorf("E2E_TEST_MODE=true is incompatible with SECURE_COOKIES=true (test-login must never be enabled in prod-shaped envs)")
	}

	return &Config{
		DatabaseURL:               dbURL,
		Port:                      port,
		LogLevel:                  level,
		SecureCookies:             secureCookies,
		CORSOrigin:                corsOrigin,
		GoogleClientID:            googleID,
		GoogleClientSecret:        googleSecret,
		GoogleRedirectURL:         googleRedirect,
		GoogleCalendarRedirectURL: googleCalRedirect,
		FrontendURL:               frontendURL,
		CookieDomain:              cookieDomain,
		ResendAPIKey:              resendKey,
		MagicLinkFromEmail:        fromEmail,
		AuthEmailDisabled:         authEmailDisabled,
		CalendarAccessSecret:      accessSecret,
		NotificationsHMACSecret:   notifSecret,
		PostHogKey:                os.Getenv("POSTHOG_KEY"),
		PostHogHost:               os.Getenv("POSTHOG_HOST"),
		PostHogPersonalKey:        os.Getenv("POSTHOG_PERSONAL_API_KEY"),
		E2ETestMode:               e2eTestMode,
		TrustedProxyDepth:         trustedProxyDepth,
	}, nil
}

func parseNonNegativeInt(s string, fallback int) (int, error) {
	if s == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q (want non-negative integer)", s)
	}
	if n < 0 {
		return 0, fmt.Errorf("invalid value %q (must be >= 0)", s)
	}
	return n, nil
}

func validateAuthEmail(disabled bool, resendKey, fromEmail, frontendURL string) error {
	anySet := resendKey != "" || fromEmail != "" || frontendURL != ""
	if disabled {
		if anySet {
			return fmt.Errorf("AUTH_EMAIL_DISABLED=true but RESEND_API_KEY/MAGIC_LINK_FROM_EMAIL/FRONTEND_URL are set — unset them or flip AUTH_EMAIL_DISABLED")
		}
		return nil
	}
	if resendKey == "" || fromEmail == "" || frontendURL == "" {
		return fmt.Errorf("RESEND_API_KEY, MAGIC_LINK_FROM_EMAIL, and FRONTEND_URL are all required (set AUTH_EMAIL_DISABLED=true to opt out)")
	}
	return nil
}

func validateGoogleOAuth(id, secret, loginRedirect, calendarRedirect string) error {
	hasID := id != ""
	hasSecret := secret != ""
	if hasID != hasSecret {
		return fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set together (or both unset to disable Google OAuth)")
	}
	if !hasID {
		if loginRedirect != "" || calendarRedirect != "" {
			return fmt.Errorf("GOOGLE_REDIRECT_URL/GOOGLE_CALENDAR_REDIRECT_URL set without GOOGLE_CLIENT_ID — add credentials or unset the redirect URLs")
		}
		return nil
	}
	if loginRedirect == "" && calendarRedirect == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID is set but neither GOOGLE_REDIRECT_URL (login) nor GOOGLE_CALENDAR_REDIRECT_URL (calendar import) is configured")
	}
	return nil
}

func parseLogLevel(s string) (slog.Level, error) {
	switch s {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL: invalid value %q (want debug|info|warn|error)", s)
	}
}

func parseBool(s string, fallback bool) (bool, error) {
	switch s {
	case "":
		return fallback, nil
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid value %q (want true|false)", s)
	}
}
