package calprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/freewhen/backend/internal/availability"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
)

// defaultGoogleFreeBusyURL is the production Google Calendar freeBusy
// endpoint. Overridable per-instance via WithGoogleFreeBusyURL for tests.
const defaultGoogleFreeBusyURL = "https://www.googleapis.com/calendar/v3/freeBusy"

// defaultGoogleAddEventURL is Google's render endpoint — opens the calendar
// UI with the event template pre-filled.
const defaultGoogleAddEventURL = "https://calendar.google.com/calendar/render"

// Google implements Provider against the Google Calendar API.
type Google struct {
	oauth        *oauth2.Config
	httpClient   *http.Client
	freeBusyURL  string
	addEventBase string
}

// GoogleOption configures optional fields on the Google provider.
type GoogleOption func(*Google)

// WithGoogleHTTPClient swaps the http client used for FreeBusy. Defaults to
// http.DefaultClient.
func WithGoogleHTTPClient(c *http.Client) GoogleOption {
	return func(g *Google) {
		if c != nil {
			g.httpClient = c
		}
	}
}

// WithGoogleOAuthEndpoint overrides the OAuth endpoint. Tests point this at an
// httptest.Server.
func WithGoogleOAuthEndpoint(ep oauth2.Endpoint) GoogleOption {
	return func(g *Google) {
		if g.oauth != nil {
			g.oauth.Endpoint = ep
		}
	}
}

// WithGoogleFreeBusyURL overrides the freeBusy endpoint. Tests point this at
// an httptest.Server.
func WithGoogleFreeBusyURL(u string) GoogleOption {
	return func(g *Google) {
		if u != "" {
			g.freeBusyURL = u
		}
	}
}

// NewGoogle constructs a Google provider. The OAuth surface (Connect /
// Callback) is only available when both clientID and redirectURL are set; the
// pure URL-builder surface (AddEventURL) works regardless so the export deep
// link keeps functioning in dev environments without OAuth credentials.
func NewGoogle(clientID, clientSecret, redirectURL string, opts ...GoogleOption) *Google {
	var oauthCfg *oauth2.Config
	if clientID != "" && redirectURL != "" {
		oauthCfg = &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"https://www.googleapis.com/auth/calendar.freebusy"},
			Endpoint:     googleoauth.Endpoint,
		}
	}
	g := &Google{
		oauth:        oauthCfg,
		httpClient:   http.DefaultClient,
		freeBusyURL:  defaultGoogleFreeBusyURL,
		addEventBase: defaultGoogleAddEventURL,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Name implements Provider.
func (g *Google) Name() string { return "google" }

// OAuthConfig implements Provider.
func (g *Google) OAuthConfig() *oauth2.Config { return g.oauth }

// FreeBusy implements Provider — calls POST /calendar/v3/freeBusy for the
// primary calendar and returns the busy intervals verbatim. The access token
// is used once and never persisted by the caller.
func (g *Google) FreeBusy(ctx context.Context, accessToken string, start, end time.Time, tz string) ([]availability.TimeRange, error) {
	body, err := json.Marshal(map[string]any{
		"timeMin":  start.UTC().Format(time.RFC3339),
		"timeMax":  end.UTC().Format(time.RFC3339),
		"timeZone": tz,
		"items":    []map[string]string{{"id": "primary"}},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal freebusy request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.freeBusyURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build freebusy request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call freebusy: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("freebusy status %d: %s", resp.StatusCode, string(snippet))
	}

	var payload struct {
		Calendars map[string]struct {
			Busy []struct {
				Start time.Time `json:"start"`
				End   time.Time `json:"end"`
			} `json:"busy"`
			Errors []struct {
				Domain string `json:"domain"`
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"calendars"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode freebusy response: %w", err)
	}

	cal, ok := payload.Calendars["primary"]
	if !ok {
		return nil, nil
	}
	if len(cal.Errors) > 0 {
		return nil, fmt.Errorf("freebusy error: %s/%s", cal.Errors[0].Domain, cal.Errors[0].Reason)
	}

	out := make([]availability.TimeRange, 0, len(cal.Busy))
	for _, b := range cal.Busy {
		out = append(out, availability.TimeRange{Start: b.Start, End: b.End})
	}
	return out, nil
}

// AddEventURL implements Provider — Google's render URL expects dates in
// YYYYMMDD/YYYYMMDD form for all-day events (end exclusive, matching iCal).
func (g *Google) AddEventURL(event ExportEvent) string {
	q := url.Values{}
	q.Set("action", "TEMPLATE")
	q.Set("text", event.Title)
	if event.AllDay {
		start := event.Date.Format("20060102")
		end := event.Date.AddDate(0, 0, 1).Format("20060102")
		q.Set("dates", start+"/"+end)
	} else {
		start := event.Date.UTC().Format("20060102T150405Z")
		end := event.Date.Add(time.Hour).UTC().Format("20060102T150405Z")
		q.Set("dates", start+"/"+end)
	}
	if event.Description != "" {
		q.Set("details", event.Description)
	}
	return g.addEventBase + "?" + q.Encode()
}
