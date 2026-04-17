package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/lib/pq"

	"github.com/freewhen/backend/internal/config"
	"github.com/freewhen/backend/internal/server"
	"github.com/freewhen/backend/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	var logger *slog.Logger
	if cfg.Env == "dev" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	if err := store.Migrate(db, logger); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	srv := server.New(db, logger, cfg.CORSOrigin)

	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Info("server listening", "addr", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
