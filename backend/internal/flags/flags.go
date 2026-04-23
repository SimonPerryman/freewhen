// Package flags provides a narrow feature-flag evaluator interface so
// handlers can gate behaviour without reaching for a global. The concrete
// implementation wraps PostHog's Go SDK; a noop fallback is used when no
// PostHog key is configured so the same code path runs in every environment.
package flags

import "context"

// Evaluator evaluates feature flags for a given distinct user id.
//
// Implementations must be safe for concurrent use.
type Evaluator interface {
	// Enabled returns whether the boolean flag is on for userID, or fallback
	// if the flag cannot be resolved (service unavailable, flag missing, etc).
	Enabled(ctx context.Context, key, userID string, fallback bool) bool

	// Variant returns the multivariate variant for userID, or fallback if the
	// flag cannot be resolved.
	Variant(ctx context.Context, key, userID, fallback string) string

	// Close releases any background resources (flag polling goroutines, HTTP
	// clients) held by the evaluator.
	Close() error
}

// Noop is an Evaluator that always returns the caller-supplied fallback.
// It's what we wire up when PostHog isn't configured, so local development
// and tests don't need a real flag service.
type Noop struct{}

func (Noop) Enabled(_ context.Context, _, _ string, fallback bool) bool {
	return fallback
}

func (Noop) Variant(_ context.Context, _, _, fallback string) string {
	return fallback
}

func (Noop) Close() error { return nil }
