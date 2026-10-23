package server

import "testing"

// Only pages of the project are opened: the call must not become a way to start anything on the computer.
func TestOpenInBrowserRefusesOtherPages(t *testing.T) {
	for _, u := range []string{"https://example.com", "file:///etc/passwd", projectURL + "evil.com", "", "javascript:alert(1)"} {
		if err := openInBrowser(u); err == nil {
			t.Errorf("%q was opened", u)
		}
	}
}
