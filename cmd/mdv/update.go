package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ernsoylu/MDView/internal/selfupdate"
)

// repoSlug is where releases come from; install.sh reads the same one.
const repoSlug = "ernsoylu/MDView"

// runUpdate implements "mdv update". With check, it only reports. It
// returns the process exit code.
func runUpdate(check bool) int {
	up := selfupdate.New(repoSlug)

	rel, err := up.Latest()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv: checking for updates:", err)
		return 1
	}
	if !selfupdate.Newer(version, rel.TagName) {
		fmt.Printf("mdv %s is already up to date (latest release is %s).\n", version, rel.TagName)
		return 0
	}
	fmt.Printf("mdv %s is available — you have %s.\n", rel.TagName, version)
	if check {
		fmt.Println(rel.HTMLURL)
		return 0
	}

	dest, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv: cannot find the running binary:", err)
		return 1
	}
	// Follow a symlink so the real file is replaced, not the link.
	if resolved, err := filepath.EvalSymlinks(dest); err == nil {
		dest = resolved
	}

	fmt.Printf("Downloading %s...\n", selfupdate.AssetName(rel.TagName))
	archive, err := up.Download(rel.TagName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv: download failed:", err)
		return 1
	}
	binary, err := selfupdate.ExtractBinary(archive)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		return 1
	}
	if err := selfupdate.Replace(dest, binary); err != nil {
		fmt.Fprintln(os.Stderr, "mdv: could not replace", dest+":", err)
		fmt.Fprintln(os.Stderr, "      if it is installed system-wide, re-run with sudo, or reinstall:")
		fmt.Fprintln(os.Stderr, "      curl -fsSL https://raw.githubusercontent.com/"+repoSlug+"/main/install.sh | sh")
		return 1
	}

	fmt.Printf("Updated to %s at %s\n", rel.TagName, dest)
	fmt.Println("Release notes:", rel.HTMLURL)
	return 0
}
