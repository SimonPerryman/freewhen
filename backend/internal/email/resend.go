package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// resendURL is the Resend API endpoint. Lifted to a package var so tests can
// point it at an httptest.Server.
var resendURL = "https://api.resend.com/emails"

// Sender sends transactional emails via Resend.
type Sender struct {
	apiKey    string
	fromEmail string
	client    *http.Client
}

// NewSender creates a Resend email sender.
func NewSender(apiKey, fromEmail string) *Sender {
	return &Sender{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		client:    &http.Client{},
	}
}

type resendRequest struct {
	From    string            `json:"from"`
	To      []string          `json:"to"`
	Subject string            `json:"subject"`
	HTML    string            `json:"html"`
	Headers map[string]string `json:"headers,omitempty"`
}

// SendTransactional sends an arbitrary HTML email with optional
// List-Unsubscribe header. listUnsubscribe should already be wrapped in
// angle brackets (e.g. "<https://example.com/unsubscribe?t=abc>") or be empty
// to skip the header. When set, List-Unsubscribe-Post is also added so
// Gmail / Apple Mail surface their native one-click button.
func (s *Sender) SendTransactional(to, subject, body, listUnsubscribe string) error {
	headers := map[string]string{}
	if listUnsubscribe != "" {
		headers["List-Unsubscribe"] = listUnsubscribe
		headers["List-Unsubscribe-Post"] = "List-Unsubscribe=One-Click"
	}
	req := resendRequest{
		From:    s.fromEmail,
		To:      []string{to},
		Subject: subject,
		HTML:    body,
		Headers: headers,
	}
	return s.post(req)
}

func (s *Sender) post(req resendRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal email request: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, resendURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create email request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("resend API returned status %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	return nil
}

// SendMagicLink sends a magic link email to the given address.
func (s *Sender) SendMagicLink(to, link string) error {
	return s.post(resendRequest{
		From:    s.fromEmail,
		To:      []string{to},
		Subject: "Sign in to FreeWhen",
		HTML: fmt.Sprintf(`<div style="font-family: system-ui, sans-serif; max-width: 400px; margin: 0 auto; padding: 40px 20px;">
  <h1 style="font-size: 24px; margin: 0 0 16px;">Sign in to FreeWhen</h1>
  <p style="color: #555; line-height: 1.5; margin: 0 0 24px;">Click the button below to sign in. This link expires in 15 minutes.</p>
  <a href="%s" style="display: inline-block; padding: 12px 24px; background: #111; color: #fff; text-decoration: none; border-radius: 8px; font-weight: 500;">Sign in</a>
  <p style="color: #999; font-size: 13px; margin: 24px 0 0;">If you didn't request this, you can safely ignore this email.</p>
</div>`, link),
	})
}
