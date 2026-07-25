// Package fetch retrieves markdown over HTTP for the remote forms of the
// command line: a URL, or a GitHub repository whose README to read.
package fetch

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MaxBytes caps a download. A markdown file past this is not something to
// read in a pager, and the cap is what stops a wrong URL pulling an ISO.
const MaxBytes = 8 << 20

// timeout bounds the whole request, so a hung server does not leave mdv
// staring at a blank terminal.
const timeout = 20 * time.Second

// readmeNames are tried in order when a repository is named without a file.
var readmeNames = []string{"README.md", "README.markdown", "readme.md", "docs/README.md"}

// IsRemote reports whether an argument is meant to be fetched rather than
// opened from disk. Anything carrying a scheme counts, including ones Get
// refuses: "mdv ftp://host/a.md" should say which schemes are fetched, not
// report a missing file by that name.
//
// A bare "owner/repo" is deliberately not remote — it is indistinguishable
// from a relative path, and guessing wrong would reach the network for a
// mistyped filename.
func IsRemote(arg string) bool {
	return strings.Contains(arg, "://") || strings.HasPrefix(arg, "github.com/")
}

// Get retrieves the document named by arg, returning its bytes and the
// name to show in the status bar.
func Get(arg string) ([]byte, string, error) {
	targets, err := resolve(arg)
	if err != nil {
		return nil, "", err
	}
	client := &http.Client{Timeout: timeout}
	var lastErr error
	for _, u := range targets {
		body, err := get(client, u)
		if err == nil {
			return body, displayName(u), nil
		}
		lastErr = err
	}
	return nil, "", lastErr
}

// resolve turns an argument into the URLs to try, in order.
func resolve(arg string) ([]string, error) {
	raw := arg
	if strings.HasPrefix(raw, "github.com/") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", arg, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%s: only http and https are fetched", arg)
	}
	if u.Host != "github.com" {
		return []string{u.String()}, nil
	}

	// github.com/owner/repo[/blob/ref/path] is a web page wrapping the file;
	// raw.githubusercontent.com serves the file itself.
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("%s: expected github.com/owner/repo", arg)
	}
	owner, repo := parts[0], parts[1]
	if len(parts) >= 5 && (parts[2] == "blob" || parts[2] == "raw") {
		ref, file := parts[3], strings.Join(parts[4:], "/")
		return []string{rawURL(owner, repo, ref, file)}, nil
	}
	if len(parts) > 2 {
		return nil, fmt.Errorf("%s: expected github.com/owner/repo or a blob URL", arg)
	}
	out := make([]string, 0, len(readmeNames))
	for _, name := range readmeNames {
		out = append(out, rawURL(owner, repo, "HEAD", name))
	}
	return out, nil
}

func rawURL(owner, repo, ref, file string) string {
	return "https://raw.githubusercontent.com/" + owner + "/" + repo + "/" + ref + "/" + file
}

func get(client *http.Client, target string) ([]byte, error) {
	resp, err := client.Get(target)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", target, resp.Status)
	}
	// One byte past the cap distinguishes "exactly at the limit" from
	// "truncated", so an oversized document is reported rather than
	// silently cut in half.
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxBytes {
		return nil, fmt.Errorf("%s: larger than %d MiB", target, MaxBytes>>20)
	}
	return body, nil
}

// displayName shortens a URL to something that fits a status bar.
func displayName(target string) string {
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	if u.Host == "raw.githubusercontent.com" {
		// /owner/repo/ref/path -> owner/repo:path
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 4 {
			return parts[0] + "/" + parts[1] + ":" + strings.Join(parts[3:], "/")
		}
	}
	return u.Host + u.Path
}
