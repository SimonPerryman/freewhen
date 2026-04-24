package email

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withStubServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	prev := resendURL
	resendURL = srv.URL
	t.Cleanup(func() { resendURL = prev })
}

func TestSendMagicLink_Success(t *testing.T) {
	var gotBody resendRequest
	var gotAuth, gotCT string

	withStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(data, &gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"id":"xyz"}`)
	})

	s := NewSender("re_secret", "noreply@example.com")
	if err := s.SendMagicLink("user@example.com", "https://example.com/link?token=abc"); err != nil {
		t.Fatalf("SendMagicLink: %v", err)
	}

	if gotAuth != "Bearer re_secret" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q", gotCT)
	}
	if gotBody.From != "noreply@example.com" {
		t.Errorf("from = %q", gotBody.From)
	}
	if len(gotBody.To) != 1 || gotBody.To[0] != "user@example.com" {
		t.Errorf("to = %v", gotBody.To)
	}
	if gotBody.Subject == "" {
		t.Error("subject should not be empty")
	}
	if !strings.Contains(gotBody.HTML, "https://example.com/link?token=abc") {
		t.Errorf("HTML body missing magic link: %q", gotBody.HTML)
	}
}

func TestSendMagicLink_APIErrorReturnsError(t *testing.T) {
	withStubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid key"}`)
	})

	s := NewSender("bad", "noreply@example.com")
	err := s.SendMagicLink("user@example.com", "https://example.com/l")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("err = %v, want to mention status code", err)
	}
}

func TestSendMagicLink_NetworkErrorReturnsError(t *testing.T) {
	// Point at a URL we don't serve.
	prev := resendURL
	resendURL = "http://127.0.0.1:1/nope"
	t.Cleanup(func() { resendURL = prev })

	s := NewSender("k", "noreply@example.com")
	if err := s.SendMagicLink("user@example.com", "https://example.com/l"); err == nil {
		t.Fatal("expected network error")
	}
}
