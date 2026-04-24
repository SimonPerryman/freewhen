package flags

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/posthog/posthog-go"
)

// posthogClient is the subset of posthog.Client we use. Having it as an
// interface lets tests substitute a fake without spinning up a real PostHog
// client or server.
type posthogClient interface {
	IsFeatureEnabled(payload posthog.FeatureFlagPayload) (interface{}, error)
	GetFeatureFlag(payload posthog.FeatureFlagPayload) (interface{}, error)
	Close() error
}

// PostHogEvaluator is an Evaluator backed by PostHog's Go SDK.
//
// Flag definitions are polled locally by the SDK (configured via PersonalApiKey
// when available), so Enabled/Variant are in-process calls rather than a
// network hop per request.
type PostHogEvaluator struct {
	client posthogClient
	logger *slog.Logger
}

// PostHogConfig carries the runtime configuration for the PostHog evaluator.
type PostHogConfig struct {
	// APIKey is the project API key (PostHog calls this the "Project API Key").
	// Required.
	APIKey string
	// Host is the PostHog API host (e.g. https://eu.i.posthog.com). Optional;
	// falls back to the SDK default when empty.
	Host string
	// PersonalAPIKey is optional; when set, the SDK polls flag definitions
	// locally for fast in-process evaluation.
	PersonalAPIKey string
}

// NewPostHog constructs a PostHog-backed Evaluator. It returns an error only
// when the SDK itself rejects the config — callers upstream of this should
// fall back to Noop when APIKey is empty.
func NewPostHog(cfg PostHogConfig, logger *slog.Logger) (*PostHogEvaluator, error) {
	c := posthog.Config{
		PersonalApiKey: cfg.PersonalAPIKey,
	}
	if cfg.Host != "" {
		c.Endpoint = cfg.Host
	}
	client, err := posthog.NewWithConfig(cfg.APIKey, c)
	if err != nil {
		return nil, fmt.Errorf("flags: posthog client: %w", err)
	}
	return &PostHogEvaluator{client: client, logger: logger}, nil
}

func (e *PostHogEvaluator) Enabled(_ context.Context, key, userID string, fallback bool) bool {
	if userID == "" {
		return fallback
	}
	v, err := e.client.IsFeatureEnabled(posthog.FeatureFlagPayload{
		Key:        key,
		DistinctId: userID,
	})
	if err != nil {
		e.logger.Warn("flag evaluation failed", "flag", key, "error", err)
		return fallback
	}
	if b, ok := v.(bool); ok {
		return b
	}
	// Multivariate flag — any non-empty variant counts as "on".
	if s, ok := v.(string); ok {
		return s != ""
	}
	return fallback
}

func (e *PostHogEvaluator) Variant(_ context.Context, key, userID, fallback string) string {
	if userID == "" {
		return fallback
	}
	v, err := e.client.GetFeatureFlag(posthog.FeatureFlagPayload{
		Key:        key,
		DistinctId: userID,
	})
	if err != nil {
		e.logger.Warn("flag evaluation failed", "flag", key, "error", err)
		return fallback
	}
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.(bool); ok {
		if b {
			return "true"
		}
		return "false"
	}
	return fallback
}

func (e *PostHogEvaluator) Close() error {
	return e.client.Close()
}
