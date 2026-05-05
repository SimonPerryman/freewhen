package handler

import (
	"time"

	"github.com/freewhen/backend/internal/ratelimit"
)

// Magic-link abuse-control budgets. Token-bucket per email (mailbomb defense)
// and per IP (cross-email enumeration / sender-reputation abuse). Both
// budgets must pass.
//
// Numbers are deliberately generous so a real user re-requesting a link
// after a typo or a delayed inbox isn't blocked, but tight enough that an
// abuser hits the wall in seconds, not hours.
const (
	magicLinkPerEmailWindow = 15 * time.Minute
	magicLinkPerEmailMax    = 5

	magicLinkPerIPWindow = 15 * time.Minute
	magicLinkPerIPMax    = 30
)

// magicLinkLimiter pairs the per-email and per-IP token buckets so the
// handler can ask "may I issue this link?" with a single call.
type magicLinkLimiter struct {
	email *ratelimit.Bucket
	ip    *ratelimit.Bucket
}

func newMagicLinkLimiter() *magicLinkLimiter {
	return &magicLinkLimiter{
		email: ratelimit.New(magicLinkPerEmailMax, magicLinkPerEmailWindow),
		ip:    ratelimit.New(magicLinkPerIPMax, magicLinkPerIPWindow),
	}
}

// allow returns true only if both budgets accept the attempt. It calls
// Allow on each bucket regardless of the other's result so a blocked
// attacker can't use the per-IP bucket as a side channel for the per-email
// state (and vice versa).
func (l *magicLinkLimiter) allow(email, ip string) bool {
	emailOK := l.email.Allow(email)
	ipOK := l.ip.Allow(ip)
	return emailOK && ipOK
}
