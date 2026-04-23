// Package google contains minimal wrappers over Google APIs. Kept small —
// we deliberately avoid pulling in the full `google.golang.org/api` client
// for a single endpoint.
package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/freewhen/backend/internal/availability"
)

// FreeBusy calls POST https://www.googleapis.com/calendar/v3/freeBusy for the
// primary calendar and returns the busy intervals verbatim. The access token
// is used once and never persisted by the caller. tz is the IANA timezone the
// caller wants Google to interpret the range in — this drives the per-day
// boundaries on Google's side too.
func FreeBusy(ctx context.Context, httpClient *http.Client, accessToken string, start, end time.Time, tz string) ([]availability.TimeRange, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	body, err := json.Marshal(map[string]any{
		"timeMin":  start.UTC().Format(time.RFC3339),
		"timeMax":  end.UTC().Format(time.RFC3339),
		"timeZone": tz,
		"items":    []map[string]string{{"id": "primary"}},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal freebusy request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.googleapis.com/calendar/v3/freeBusy", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build freebusy request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
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
