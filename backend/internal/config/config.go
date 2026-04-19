package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string
	Port               string
	Env                string
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
	if os.Getenv("ENV") != "prod" {
		_ = godotenv.Load(".env.local", ".env")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "dev"
	}

	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:3000"
	}

	return &Config{
		DatabaseURL:        dbURL,
		Port:               port,
		Env:                env,
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
