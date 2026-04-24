package access

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

const testSecret = "test-secret-at-least-32-bytes-long-xx"

func TestIssueAndVerify_RoundTrip(t *testing.T) {
	tok := IssueToken("cal1", 3, testSecret, time.Hour)
	id, epoch, err := VerifyToken(tok, testSecret)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if id != "cal1" {
		t.Fatalf("calendar ID = %q, want cal1", id)
	}
	if epoch != 3 {
		t.Fatalf("epoch = %d, want 3", epoch)
	}
}

func TestVerify_RejectsTampered(t *testing.T) {
	tok := IssueToken("cal1", 0, testSecret, time.Hour)
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 {
		t.Fatalf("unexpected token shape: %d parts", len(parts))
	}
	parts[3] = "not-the-real-signature"
	tampered := base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, "|")))
	if _, _, err := VerifyToken(tampered, testSecret); err == nil {
		t.Fatal("expected error from tampered token")
	}
}

func TestVerify_RejectsWrongSecret(t *testing.T) {
	tok := IssueToken("cal1", 0, testSecret, time.Hour)
	if _, _, err := VerifyToken(tok, "another-secret-at-least-32-bytes-long"); err == nil {
		t.Fatal("expected error from wrong-secret verification")
	}
}

func TestVerify_RejectsExpired(t *testing.T) {
	tok := IssueToken("cal1", 0, testSecret, -time.Second)
	if _, _, err := VerifyToken(tok, testSecret); err == nil {
		t.Fatal("expected error from expired token")
	}
}

func TestVerify_RejectsMalformed(t *testing.T) {
	if _, _, err := VerifyToken("not-base64!!!", testSecret); err == nil {
		t.Fatal("expected error from malformed token")
	}
}
