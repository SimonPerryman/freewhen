package flags

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/posthog/posthog-go"
)

type fakeClient struct {
	isEnabledValue interface{}
	isEnabledErr   error
	getFlagValue   interface{}
	getFlagErr     error

	lastIsEnabledKey string
	lastIsEnabledID  string
	lastGetFlagKey   string
	lastGetFlagID    string
	closeCalls       int
}

func (f *fakeClient) IsFeatureEnabled(p posthog.FeatureFlagPayload) (interface{}, error) {
	f.lastIsEnabledKey = p.Key
	f.lastIsEnabledID = p.DistinctId
	return f.isEnabledValue, f.isEnabledErr
}

func (f *fakeClient) GetFeatureFlag(p posthog.FeatureFlagPayload) (interface{}, error) {
	f.lastGetFlagKey = p.Key
	f.lastGetFlagID = p.DistinctId
	return f.getFlagValue, f.getFlagErr
}

func (f *fakeClient) Close() error {
	f.closeCalls++
	return nil
}

func newEvaluator(c *fakeClient) *PostHogEvaluator {
	return &PostHogEvaluator{
		client: c,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestEnabled_BoolTrue(t *testing.T) {
	c := &fakeClient{isEnabledValue: true}
	e := newEvaluator(c)
	if !e.Enabled(context.Background(), "flag", "user-1", false) {
		t.Error("Enabled should return true")
	}
	if c.lastIsEnabledKey != "flag" {
		t.Errorf("key = %q", c.lastIsEnabledKey)
	}
	if c.lastIsEnabledID != "user-1" {
		t.Errorf("distinctId = %q", c.lastIsEnabledID)
	}
}

func TestEnabled_BoolFalse(t *testing.T) {
	e := newEvaluator(&fakeClient{isEnabledValue: false})
	if e.Enabled(context.Background(), "flag", "user-1", true) {
		t.Error("Enabled should return false")
	}
}

func TestEnabled_MultivariateNonEmptyIsOn(t *testing.T) {
	e := newEvaluator(&fakeClient{isEnabledValue: "variant-a"})
	if !e.Enabled(context.Background(), "flag", "user-1", false) {
		t.Error("non-empty variant should count as on")
	}
}

func TestEnabled_MultivariateEmptyIsOff(t *testing.T) {
	e := newEvaluator(&fakeClient{isEnabledValue: ""})
	if e.Enabled(context.Background(), "flag", "user-1", true) {
		t.Error("empty variant should count as off")
	}
}

func TestEnabled_ErrorReturnsFallback(t *testing.T) {
	e := newEvaluator(&fakeClient{isEnabledErr: errors.New("nope")})
	if !e.Enabled(context.Background(), "flag", "user-1", true) {
		t.Error("error should return fallback=true")
	}
	if e.Enabled(context.Background(), "flag", "user-1", false) {
		t.Error("error should return fallback=false")
	}
}

func TestEnabled_EmptyUserReturnsFallbackWithoutCall(t *testing.T) {
	c := &fakeClient{isEnabledValue: true}
	e := newEvaluator(c)
	if e.Enabled(context.Background(), "flag", "", false) {
		t.Error("empty userID should return fallback")
	}
	if c.lastIsEnabledKey != "" {
		t.Error("should not call client with empty userID")
	}
}

func TestEnabled_UnknownTypeReturnsFallback(t *testing.T) {
	e := newEvaluator(&fakeClient{isEnabledValue: 42})
	if !e.Enabled(context.Background(), "flag", "u", true) {
		t.Error("unknown type should return fallback")
	}
}

func TestVariant_StringPassesThrough(t *testing.T) {
	e := newEvaluator(&fakeClient{getFlagValue: "variant-a"})
	if got := e.Variant(context.Background(), "flag", "u", "default"); got != "variant-a" {
		t.Errorf("Variant = %q, want variant-a", got)
	}
}

func TestVariant_BoolMapsToTrueFalse(t *testing.T) {
	e := newEvaluator(&fakeClient{getFlagValue: true})
	if got := e.Variant(context.Background(), "flag", "u", "default"); got != "true" {
		t.Errorf("bool true -> %q, want \"true\"", got)
	}
	e = newEvaluator(&fakeClient{getFlagValue: false})
	if got := e.Variant(context.Background(), "flag", "u", "default"); got != "false" {
		t.Errorf("bool false -> %q, want \"false\"", got)
	}
}

func TestVariant_ErrorReturnsFallback(t *testing.T) {
	e := newEvaluator(&fakeClient{getFlagErr: errors.New("nope")})
	if got := e.Variant(context.Background(), "flag", "u", "fallback"); got != "fallback" {
		t.Errorf("on error = %q, want fallback", got)
	}
}

func TestVariant_EmptyUserReturnsFallback(t *testing.T) {
	c := &fakeClient{getFlagValue: "variant-a"}
	e := newEvaluator(c)
	if got := e.Variant(context.Background(), "flag", "", "default"); got != "default" {
		t.Errorf("empty userID -> %q, want default", got)
	}
	if c.lastGetFlagKey != "" {
		t.Error("should not call client with empty userID")
	}
}

func TestClose_DelegatesToClient(t *testing.T) {
	c := &fakeClient{}
	e := newEvaluator(c)
	if err := e.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if c.closeCalls != 1 {
		t.Errorf("Close called %d times, want 1", c.closeCalls)
	}
}

func TestNoop(t *testing.T) {
	var e Evaluator = Noop{}
	if !e.Enabled(context.Background(), "k", "u", true) {
		t.Error("Noop.Enabled should return fallback=true")
	}
	if e.Enabled(context.Background(), "k", "u", false) {
		t.Error("Noop.Enabled should return fallback=false")
	}
	if got := e.Variant(context.Background(), "k", "u", "fb"); got != "fb" {
		t.Errorf("Noop.Variant = %q, want fb", got)
	}
	if err := e.Close(); err != nil {
		t.Errorf("Noop.Close: %v", err)
	}
}
