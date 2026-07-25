// Package selfupdate replaces the running mdv binary with the newest
// published release. It mirrors install.sh's contract — the same asset
// names, the same checksums.txt — so a binary installed either way can be
// updated either way.
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	// maxAsset caps a download. Release archives are a few MB; anything
	// past this is not an mdv build.
	maxAsset = 64 << 20
	timeout  = 60 * time.Second
)

// Release is the part of the GitHub release API this needs.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Name    string `json:"name"`
}

// Updater talks to a release host. The zero value is not usable; use New.
type Updater struct {
	Repo     string // "owner/name"
	APIBase  string // https://api.github.com
	DownBase string // https://github.com
	Client   *http.Client
}

func New(repo string) *Updater {
	return &Updater{
		Repo:     repo,
		APIBase:  "https://api.github.com",
		DownBase: "https://github.com",
		Client:   &http.Client{Timeout: timeout},
	}
}

// Latest returns the newest published release.
func (u *Updater) Latest() (Release, error) {
	var rel Release
	body, err := u.get(u.APIBase + "/repos/" + u.Repo + "/releases/latest")
	if err != nil {
		return rel, err
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return rel, fmt.Errorf("release feed: %w", err)
	}
	if rel.TagName == "" {
		return rel, fmt.Errorf("release feed carried no tag")
	}
	return rel, nil
}

// AssetName is the archive this platform needs from a given tag.
func AssetName(tag string) string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("mdv_%s_%s_%s.%s", strings.TrimPrefix(tag, "v"), runtime.GOOS, runtime.GOARCH, ext)
}

// Download fetches the platform's archive for tag and verifies it against
// the release's checksums.txt. An unverified binary is not worth having:
// this replaces the program the user runs.
func (u *Updater) Download(tag string) ([]byte, error) {
	asset := AssetName(tag)
	base := u.DownBase + "/" + u.Repo + "/releases/download/" + tag

	archive, err := u.get(base + "/" + asset)
	if err != nil {
		return nil, fmt.Errorf("%s: %w (no build for %s/%s?)", asset, err, runtime.GOOS, runtime.GOARCH)
	}
	sums, err := u.get(base + "/checksums.txt")
	if err != nil {
		return nil, fmt.Errorf("checksums.txt: %w", err)
	}
	want, err := checksumFor(string(sums), asset)
	if err != nil {
		return nil, err
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return nil, fmt.Errorf("checksum mismatch for %s", asset)
	}
	return archive, nil
}

// checksumFor pulls one asset's digest out of a checksums.txt.
func checksumFor(sums, asset string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("%s is not listed in checksums.txt", asset)
}

// ExtractBinary pulls the mdv executable out of a release archive.
func ExtractBinary(archive []byte) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return fromZip(archive)
	}
	return fromTarGz(archive)
}

func fromTarGz(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(h.Name) == "mdv" && h.Typeflag == tar.TypeReg {
			return io.ReadAll(io.LimitReader(tr, maxAsset))
		}
	}
	return nil, fmt.Errorf("archive did not contain the mdv binary")
}

func fromZip(archive []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) == "mdv.exe" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return io.ReadAll(io.LimitReader(rc, maxAsset))
		}
	}
	return nil, fmt.Errorf("archive did not contain mdv.exe")
}

// Replace installs binary at dest.
//
// The new file is written beside the target and renamed over it: rename is
// atomic within a filesystem, and on unix replacing a running program's
// path is fine because the running image holds the old inode. Windows will
// not rename over a file in use, so the old one is moved aside first.
func Replace(dest string, binary []byte) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".mdv-update-*")
	if err != nil {
		return fmt.Errorf("%s is not writable: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once renamed away

	if _, err := tmp.Write(binary); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		old := dest + ".old"
		_ = os.Remove(old)
		if err := os.Rename(dest, old); err != nil {
			return err
		}
		if err := os.Rename(tmpName, dest); err != nil {
			_ = os.Rename(old, dest) // put it back
			return err
		}
		_ = os.Remove(old)
		return nil
	}
	return os.Rename(tmpName, dest)
}

// Newer reports whether tag names a later version than current. A build
// without a stamped version — "dev", or anything unparsable — counts as
// older, so a source build can still pull a release.
func Newer(current, tag string) bool {
	c, okC := parseVersion(current)
	t, okT := parseVersion(tag)
	if !okT {
		return false // nothing to update to
	}
	if !okC {
		return true
	}
	for i := 0; i < 3; i++ {
		if t[i] != c[i] {
			return t[i] > c[i]
		}
	}
	return false
}

// parseVersion reads a leading MAJOR.MINOR.PATCH, ignoring a "v" prefix and
// any pre-release or build suffix.
func parseVersion(s string) ([3]int, bool) {
	var out [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || parts[0] == "" {
		return out, false
	}
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

func (u *Updater) get(url string) ([]byte, error) {
	resp, err := u.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAsset+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxAsset {
		return nil, fmt.Errorf("response larger than %d MiB", maxAsset>>20)
	}
	return body, nil
}
