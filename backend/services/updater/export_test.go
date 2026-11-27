package updater

import (
	"net/url"
	"strings"
)

// AllowTestURLHost registers an httptest server host for unit tests without exposing it to Wails bindings.
func (s *Service) AllowTestURLHost(rawURL string) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return
	}
	s.mu.Lock()
	s.allowedTestHosts[host] = true
	s.mu.Unlock()
}

// SetSkipLaunchInTest prevents ApplyUpdate from spawning external processes during unit tests.
func (s *Service) SetSkipLaunchInTest(skip bool) {
	s.mu.Lock()
	s.skipLaunchInTest = skip
	s.mu.Unlock()
}
