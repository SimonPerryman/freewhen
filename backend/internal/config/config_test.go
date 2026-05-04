package config

import (
	"log/slog"
	"strings"
	"testing"
)

// setMinimalEnv clears/sets all config-relevant env vars to a known-good baseline
// so tests aren't contaminated by the developer's shell environment. Individual
// tests can override specific vars with t.Setenv before calling Load.
func setMinimalEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"DATABASE_URL", "PORT", "LOG_LEVEL", "SECURE_COOKIES", "CORS_ORIGIN",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL",
		"GOOGLE_CALENDAR_REDIRECT_URL", "FRONTEND_URL", "COOKIE_DOMAIN",
		"RESEND_API_KEY", "MAGIC_LINK_FROM_EMAIL", "AUTH_EMAIL_DISABLED",
		"CALENDAR_ACCESS_SECRET", "POSTHOG_KEY", "POSTHOG_HOST",
		"POSTHOG_PERSONAL_API_KEY", "E2E_TEST_MODE",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("CALENDAR_ACCESS_SECRET", strings.Repeat("x", 32))
	t.Setenv("SECURE_COOKIES", "false")
	t.Setenv("AUTH_EMAIL_DISABLED", "true")
}

func TestLoad_Defaults(t *testing.T) {
	setMinimalEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DatabaseURL != "postgres://test" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default 8080", cfg.Port)
	}
	if cfg.CORSOrigin != "http://localhost:3000" {
		t.Errorf("CORSOrigin = %q, want default localhost:3000", cfg.CORSOrigin)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if cfg.SecureCookies {
		t.Error("SecureCookies should be false")
	}
	if !cfg.AuthEmailDisabled {
		t.Error("AuthEmailDisabled should be true")
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL unset")
	}
}

func TestLoad_ShortAccessSecret(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("CALENDAR_ACCESS_SECRET", "too-short")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for secret < 32 bytes")
	}
}

func TestLoad_Overrides(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("PORT", "9090")
	t.Setenv("CORS_ORIGIN", "https://example.com")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Port = %q", cfg.Port)
	}
	if cfg.CORSOrigin != "https://example.com" {
		t.Errorf("CORSOrigin = %q", cfg.CORSOrigin)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
}

func TestLoad_SecureCookiesRequiresCookieDomain(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("SECURE_COOKIES", "true")
	// COOKIE_DOMAIN stays empty

	if _, err := Load(); err == nil {
		t.Fatal("expected error: SECURE_COOKIES=true without COOKIE_DOMAIN")
	}

	t.Setenv("COOKIE_DOMAIN", "example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with COOKIE_DOMAIN: %v", err)
	}
	if !cfg.SecureCookies {
		t.Error("SecureCookies should be true")
	}
	if cfg.CookieDomain != "example.com" {
		t.Errorf("CookieDomain = %q", cfg.CookieDomain)
	}
}

func TestLoad_AuthEmailEnabledRequiresAllKeys(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("AUTH_EMAIL_DISABLED", "false")
	// Missing RESEND_API_KEY / MAGIC_LINK_FROM_EMAIL / FRONTEND_URL

	if _, err := Load(); err == nil {
		t.Fatal("expected error when email enabled without keys")
	}

	t.Setenv("RESEND_API_KEY", "re_key")
	t.Setenv("MAGIC_LINK_FROM_EMAIL", "noreply@example.com")
	t.Setenv("FRONTEND_URL", "https://example.com")
	if _, err := Load(); err != nil {
		t.Fatalf("Load with all keys: %v", err)
	}
}

func TestLoad_AuthEmailDisabledRejectsPartialKeys(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("AUTH_EMAIL_DISABLED", "true")
	t.Setenv("RESEND_API_KEY", "re_key") // should not be set when disabled

	if _, err := Load(); err == nil {
		t.Fatal("expected error: AUTH_EMAIL_DISABLED=true with keys set")
	}
}

func TestLoad_GoogleOAuth(t *testing.T) {
	t.Run("ID without Secret fails", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("GOOGLE_CLIENT_ID", "client-id")
		if _, err := Load(); err == nil {
			t.Fatal("expected error: ID without Secret")
		}
	})

	t.Run("Credentials without redirect URLs fails", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("GOOGLE_CLIENT_ID", "client-id")
		t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
		if _, err := Load(); err == nil {
			t.Fatal("expected error: credentials without redirects")
		}
	})

	t.Run("Redirect without credentials fails", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("GOOGLE_REDIRECT_URL", "https://example.com/cb")
		if _, err := Load(); err == nil {
			t.Fatal("expected error: redirect without credentials")
		}
	})

	t.Run("Full login config ok", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("GOOGLE_CLIENT_ID", "client-id")
		t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
		t.Setenv("GOOGLE_REDIRECT_URL", "https://example.com/cb")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.GoogleClientID != "client-id" {
			t.Errorf("GoogleClientID = %q", cfg.GoogleClientID)
		}
	})
}

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		in   string
		want slog.Level
		err  bool
	}{
		{"", slog.LevelInfo, false},
		{"info", slog.LevelInfo, false},
		{"debug", slog.LevelDebug, false},
		{"warn", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"bogus", 0, true},
	}
	for _, c := range cases {
		got, err := parseLogLevel(c.in)
		if c.err {
			if err == nil {
				t.Errorf("parseLogLevel(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseLogLevel(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseBool(t *testing.T) {
	cases := []struct {
		in       string
		fallback bool
		want     bool
		err      bool
	}{
		{"", true, true, false},
		{"", false, false, false},
		{"true", false, true, false},
		{"1", false, true, false},
		{"false", true, false, false},
		{"0", true, false, false},
		{"yes", false, false, true},
	}
	for _, c := range cases {
		got, err := parseBool(c.in, c.fallback)
		if c.err {
			if err == nil {
				t.Errorf("parseBool(%q, %v): expected error", c.in, c.fallback)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseBool(%q, %v): %v", c.in, c.fallback, err)
		}
		if got != c.want {
			t.Errorf("parseBool(%q, %v) = %v, want %v", c.in, c.fallback, got, c.want)
		}
	}
}

func TestLoad_E2ETestMode_RefusesWithSecureCookies(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("SECURE_COOKIES", "true")
	t.Setenv("COOKIE_DOMAIN", "example.com")
	t.Setenv("E2E_TEST_MODE", "true")

	if _, err := Load(); err == nil {
		t.Fatal("Load: expected error when E2E_TEST_MODE=true and SECURE_COOKIES=true")
	}
}

func TestLoad_E2ETestMode_OKWithInsecureCookies(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("E2E_TEST_MODE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.E2ETestMode {
		t.Error("E2ETestMode should be true")
	}
}
