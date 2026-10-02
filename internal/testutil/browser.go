// Package testutil provides helpers for tests that drive a real headless browser.
package testutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-rod/rod/lib/launcher"
)

// RequireBrowser skips the test in -short mode or when no local Chrome/Chromium
// is installed (rod would otherwise try to download one).
func RequireBrowser(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping browser test in -short mode")
	}
	if _, found := launcher.LookPath(); !found {
		t.Skip("no Chrome/Chromium found on this machine")
	}
}

// ServeHTML serves html at the root of a test server and returns its URL
func ServeHTML(t *testing.T, html string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}
