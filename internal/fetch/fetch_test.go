package fetch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsRemote(t *testing.T) {
	cases := map[string]bool{
		"https://example.com/a.md": true,
		"http://example.com/a.md":  true,
		"github.com/owner/repo":    true,
		"./local.md":               false,
		"docs/guide.md":            false,
		"owner/repo":               false, // ambiguous with a relative path
		"/abs/path.md":             false,
		"file:///etc/passwd":       true, // fetched=false, but Get names the reason
		"ftp://example.com/a.md":   true, // ditto
		"README.md":                false,
		"https-not-really/a.md":    false,
	}
	for arg, want := range cases {
		if got := IsRemote(arg); got != want {
			t.Errorf("IsRemote(%q) = %v, want %v", arg, got, want)
		}
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		arg  string
		want []string
	}{
		{"https://example.com/a.md", []string{"https://example.com/a.md"}},
		{
			"github.com/owner/repo/blob/main/docs/guide.md",
			[]string{"https://raw.githubusercontent.com/owner/repo/main/docs/guide.md"},
		},
		{
			"https://github.com/owner/repo/blob/v1.2.3/README.md",
			[]string{"https://raw.githubusercontent.com/owner/repo/v1.2.3/README.md"},
		},
	}
	for _, tc := range cases {
		got, err := resolve(tc.arg)
		if err != nil {
			t.Errorf("resolve(%q): %v", tc.arg, err)
			continue
		}
		if strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("resolve(%q) = %q, want %q", tc.arg, got, tc.want)
		}
	}

	// A bare repository tries the usual README spellings.
	got, err := resolve("github.com/owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(readmeNames) || !strings.HasSuffix(got[0], "/HEAD/README.md") {
		t.Errorf("bare repo resolved to %q", got)
	}

	for _, bad := range []string{"ftp://example.com/a.md", "github.com/owner", "github.com/o/r/tree/main"} {
		if _, err := resolve(bad); err == nil {
			t.Errorf("resolve(%q) should have failed", bad)
		}
	}
}

func TestGetFallsThroughReadmeNames(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "second.md") {
			_, _ = fmt.Fprint(w, "# found\n")
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := srv.Client()
	if _, err := get(client, srv.URL+"/first.md"); err == nil {
		t.Error("a 404 should be an error")
	}
	body, err := get(client, srv.URL+"/second.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "# found\n" {
		t.Errorf("body = %q", body)
	}
	if len(asked) != 2 {
		t.Errorf("asked for %v, want two requests", asked)
	}
}

// TestGetRefusesOversized is the guard against a wrong URL pulling
// something enormous into a pager.
func TestGetRefusesOversized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := strings.Repeat("x", 1<<20)
		for i := 0; i < (MaxBytes>>20)+2; i++ {
			_, _ = fmt.Fprint(w, chunk)
		}
	}))
	defer srv.Close()

	_, err := get(srv.Client(), srv.URL+"/big.md")
	if err == nil {
		t.Fatal("an oversized document should be refused")
	}
	if !strings.Contains(err.Error(), "larger than") {
		t.Errorf("error = %q, want it to name the size limit", err)
	}
}

// TestGetAcceptsExactlyTheLimit checks the cap is not off by one.
func TestGetAcceptsExactlyTheLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, MaxBytes))
	}))
	defer srv.Close()

	body, err := get(srv.Client(), srv.URL+"/edge.md")
	if err != nil {
		t.Fatalf("a document exactly at the limit should be accepted: %v", err)
	}
	if len(body) != MaxBytes {
		t.Errorf("got %d bytes, want %d", len(body), MaxBytes)
	}
}

func TestDisplayName(t *testing.T) {
	cases := map[string]string{
		"https://raw.githubusercontent.com/owner/repo/HEAD/README.md": "owner/repo:README.md",
		"https://raw.githubusercontent.com/o/r/main/docs/a.md":        "o/r:docs/a.md",
		"https://example.com/notes/a.md":                              "example.com/notes/a.md",
	}
	for in, want := range cases {
		if got := displayName(in); got != want {
			t.Errorf("displayName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestGetNamesUnsupportedScheme: a scheme mdv does not fetch should say so
// rather than be handed to the filesystem as a filename.
func TestGetNamesUnsupportedScheme(t *testing.T) {
	for _, arg := range []string{"ftp://example.com/a.md", "file:///etc/passwd"} {
		_, _, err := Get(arg)
		if err == nil {
			t.Fatalf("Get(%q) should have failed", arg)
		}
		if !strings.Contains(err.Error(), "only http and https") {
			t.Errorf("Get(%q) error = %q, want it to name the supported schemes", arg, err)
		}
	}
}
