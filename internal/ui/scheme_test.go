package ui

import (
	"strings"
	"testing"
)

func TestWebScheme(t *testing.T) {
	cases := []struct {
		target string
		want   bool
	}{
		{"https://example.com", true},
		{"http://example.com/a?b=c", true},
		{"HTTPS://EXAMPLE.COM", true},
		{"mailto:someone@example.com", true},
		{"file:///etc/passwd", false},
		{"vscode://file/etc/passwd", false},
		{"ms-msdt://id", false},
		{"smb://host/share", false},
		{"javascript:alert(1)", false},
		{"data:text/html,<script>", false},
		{"./relative.md", false},
	}
	for _, tc := range cases {
		if got := webScheme(tc.target); got != tc.want {
			t.Errorf("webScheme(%q) = %v, want %v", tc.target, got, tc.want)
		}
	}
}

// TestFollowLinkRefusesNonWebScheme checks the routing, not just the
// predicate: a file:// target must never reach the system opener.
func TestFollowLinkRefusesNonWebScheme(t *testing.T) {
	m := newLinkModel(t)
	opened := ""
	m.opener = func(target string) error { opened = target; return nil }

	if cmd := m.followLink("file:///etc/passwd"); cmd != nil {
		cmd()
	}
	if opened != "" {
		t.Errorf("opener was handed %q; it must refuse non-web schemes", opened)
	}
	if !strings.Contains(m.flash, "refusing") {
		t.Errorf("expected a refusal in the status bar, got %q", m.flash)
	}

	// An http link through the same path must still open.
	if cmd := m.followLink("https://example.com/ok"); cmd != nil {
		cmd()
	}
	if opened != "https://example.com/ok" {
		t.Errorf("opener got %q, want the https link", opened)
	}
}
