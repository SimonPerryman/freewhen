package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string
	Port               string
	LogLevel           slog.Level
	SecureCookies      bool
	CORSOrigin         string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	FrontendURL        string
	CookieDomain       string
	ResendAPIKey       string
	MagicLinkFromEmail string
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

	return &Config{
		DatabaseURL:        dbURL,
		Port:               port,
		LogLevel:           level,
		SecureCookies:      secureCookies,
		CORSOrigin:         corsOrigin,
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		FrontendURL:        os.Getenv("FRONTEND_URL"),
		CookieDomain:       os.Getenv("COOKIE_DOMAIN"),
		ResendAPIKey:       os.Getenv("RESEND_API_KEY"),
		MagicLinkFromEmail: os.Getenv("MAGIC_LINK_FROM_EMAIL"),
	}, nil
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
