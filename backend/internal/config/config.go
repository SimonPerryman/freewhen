package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
	Env         string
	CORSOrigin  string
}

func Load() (*Config, error) {
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
		corsOrigin = "*"
	}

	return &Config{
		DatabaseURL: dbURL,
		Port:        port,
		Env:         env,
		CORSOrigin:  corsOrigin,
	}, nil
}
