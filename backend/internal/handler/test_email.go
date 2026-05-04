package handler

import (
	"net/http"
	"sync"
)

// CapturingEmailSender stores the most recent magic-link URL per email so
// the e2e suite can fetch it without running a real SMTP server. Only wired
// in when config.E2ETestMode is true.
type CapturingEmailSender struct {
	mu    sync.RWMutex
	links map[string]string
}

func NewCapturingEmailSender() *CapturingEmailSender {
	return &CapturingEmailSender{links: make(map[string]string)}
}

func (c *CapturingEmailSender) SendMagicLink(to, link string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.links[to] = link
	return nil
}

func (c *CapturingEmailSender) Latest(email string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	link, ok := c.links[email]
	return link, ok
}

// LatestMagicLink returns `{"link": "..."}` for the requested email, or 404
// if none has been issued. Wired only when E2ETestMode is true.
func (c *CapturingEmailSender) LatestMagicLink(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "email query param required")
		return
	}
	link, ok := c.Latest(email)
	if !ok {
		respondError(w, http.StatusNotFound, "not_found", "no magic link issued for that email")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"link": link})
}
