package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		current, tag string
		want         bool
	}{
		{"1.1.0", "v1.2.0", true},
		{"v1.1.0", "v1.1.1", true},
		{"v1.1.0", "v2.0.0", true},
		{"v1.2.0", "v1.2.0", false},
		{"v1.2.0", "v1.1.9", false},
		{"v2.0.0", "v1.9.9", false},
		// A source build has no stamped version and should still be able to
		// pull a release.
		{"dev", "v1.2.0", true},
		{"", "v1.2.0", true},
		// Nothing sensible to update *to* is never an update.
		{"v1.1.0", "not-a-version", false},
		{"dev", "dev", false},
		// Suffixes are ignored, not treated as bigger.
		{"v1.2.0", "v1.2.0-rc1", false},
		{"v1.2.0-rc1", "v1.2.0", false},
		{"v1.1.0", "v1.2.0-rc1", true},
	}
	for _, tc := range cases {
		if got := Newer(tc.current, tc.tag); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.current, tc.tag, got, tc.want)
		}
	}
}

func TestAssetNameMatchesInstallScript(t *testing.T) {
	// install.sh builds mdv_${version}_${goos}_${goarch}.tar.gz; drifting
	// from that would leave one installer working and the other not.
	got := AssetName("v1.2.0")
	want := fmt.Sprintf("mdv_1.2.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		want = fmt.Sprintf("mdv_1.2.0_%s_%s.zip", runtime.GOOS, runtime.GOARCH)
	}
	if got != want {
		t.Errorf("AssetName = %q, want %q", got, want)
	}
}

// tarGz builds a release-shaped archive holding the given binary bytes.
func tarGz(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name string
		body []byte
	}{
		{"LICENSE", []byte("MIT")},
		{"mdv", binary},
		{"docs/mdv.1", []byte(".TH MDV 1")},
	} {
		if err := tw.WriteHeader(&tar.Header{
			Name: f.name, Mode: 0o755, Size: int64(len(f.body)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(f.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// releaseServer stands in for GitHub: the latest-release feed, the asset,
// and checksums.txt.
func releaseServer(t *testing.T, tag string, archive []byte, corruptSum bool) *httptest.Server {
	t.Helper()
	asset := AssetName(tag)
	sum := sha256.Sum256(archive)
	digest := hex.EncodeToString(sum[:])
	if corruptSum {
		digest = strings.Repeat("0", 64)
	}
	sums := fmt.Sprintf("%s  mdv_x_other_arch.tar.gz\n%s  %s\n", strings.Repeat("a", 64), digest, asset)

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `{"tag_name":%q,"html_url":"https://example.com/rel","name":%q}`, tag, tag)
	})
	mux.HandleFunc("/o/r/releases/download/"+tag+"/"+asset, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/o/r/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sums))
	})
	return httptest.NewServer(mux)
}

func updaterFor(srv *httptest.Server) *Updater {
	u := New("o/r")
	u.APIBase = srv.URL
	u.DownBase = srv.URL
	u.Client = srv.Client()
	return u
}

func TestLatest(t *testing.T) {
	srv := releaseServer(t, "v9.9.9", []byte("x"), false)
	defer srv.Close()
	rel, err := updaterFor(srv).Latest()
	if err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "v9.9.9" || rel.HTMLURL != "https://example.com/rel" {
		t.Errorf("got %+v", rel)
	}
}

// TestUpdateEndToEnd walks the whole path: fetch, verify, extract, replace.
func TestUpdateEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture archive is a tar.gz")
	}
	const tag = "v9.9.9"
	newBinary := []byte("#!/bin/sh\necho updated\n")
	srv := releaseServer(t, tag, tarGz(t, newBinary), false)
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "mdv")
	if err := os.WriteFile(dest, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	u := updaterFor(srv)
	archive, err := u.Download(tag)
	if err != nil {
		t.Fatal(err)
	}
	binary, err := ExtractBinary(archive)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(binary, newBinary) {
		t.Fatalf("extracted %q, want the mdv entry", binary)
	}
	if err := Replace(dest, binary); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newBinary) {
		t.Errorf("destination holds %q", got)
	}
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("replaced binary is not executable: %v", fi.Mode())
	}
	// The temp file must not be left lying next to it.
	entries, _ := os.ReadDir(filepath.Dir(dest))
	if len(entries) != 1 {
		t.Errorf("update left %d files behind, want just the binary", len(entries))
	}
}

// TestDownloadRejectsBadChecksum is the guard that matters most here: this
// replaces the program the user runs.
func TestDownloadRejectsBadChecksum(t *testing.T) {
	const tag = "v9.9.9"
	srv := releaseServer(t, tag, tarGz(t, []byte("payload")), true)
	defer srv.Close()

	_, err := updaterFor(srv).Download(tag)
	if err == nil {
		t.Fatal("a mismatched checksum should abort the update")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error = %q, want it to name the mismatch", err)
	}
}

func TestDownloadMissingAsset(t *testing.T) {
	const tag = "v9.9.9"
	srv := releaseServer(t, tag, tarGz(t, []byte("payload")), false)
	defer srv.Close()

	// A tag the server has no asset for.
	_, err := updaterFor(srv).Download("v0.0.1")
	if err == nil {
		t.Fatal("a missing asset should be an error")
	}
	if !strings.Contains(err.Error(), runtime.GOOS) {
		t.Errorf("error = %q, want it to name the platform", err)
	}
}

func TestChecksumFor(t *testing.T) {
	sums := "aaa  mdv_1.0.0_linux_amd64.tar.gz\nbbb  mdv_1.0.0_darwin_arm64.tar.gz\n"
	if got, err := checksumFor(sums, "mdv_1.0.0_darwin_arm64.tar.gz"); err != nil || got != "bbb" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := checksumFor(sums, "mdv_1.0.0_freebsd_amd64.tar.gz"); err == nil {
		t.Error("an unlisted asset should be an error")
	}
}

func TestExtractBinaryMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture archive is a tar.gz")
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "LICENSE", Mode: 0o644, Size: 3, Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte("MIT"))
	_ = tw.Close()
	_ = gz.Close()

	if _, err := ExtractBinary(buf.Bytes()); err == nil {
		t.Error("an archive without the binary should be an error")
	}
}

// TestReplaceReportsUnwritableDir covers the system-wide install case,
// where the fix is sudo rather than a retry.
func TestReplaceReportsUnwritableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "mdv")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0o700) }()

	err := Replace(dest, []byte("new"))
	if err == nil {
		t.Fatal("replacing into an unwritable directory should fail")
	}
	if !strings.Contains(err.Error(), "not writable") {
		t.Errorf("error = %q, want it to say the directory is not writable", err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "old" {
		t.Errorf("a failed update must leave the old binary intact, got %q", got)
	}
}
