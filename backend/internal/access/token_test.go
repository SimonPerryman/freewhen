package access

import (
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
	// Flip a byte.
	tampered := tok[:len(tok)-1] + strings.Map(func(r rune) rune {
		if r == 'a' {
			return 'b'
		}
		return 'a'
	}, string(tok[len(tok)-1:]))
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
