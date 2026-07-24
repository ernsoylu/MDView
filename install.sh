#!/bin/sh
# mdv installer — https://github.com/ernsoylu/MDView
#
#   curl -fsSL https://raw.githubusercontent.com/ernsoylu/MDView/main/install.sh | sh
#
# Detects the OS (Linux, macOS, FreeBSD) and architecture, downloads the
# latest release, verifies its checksum, installs to ~/.local/bin (override
# with MDV_INSTALL_DIR), installs the man page, and makes sure the install
# directory is on PATH in your shell rc.
set -eu

REPO="ernsoylu/MDView"
INSTALL_DIR="${MDV_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
die() { printf 'mdv installer: %s\n' "$*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar >/dev/null 2>&1 || die "tar is required"

os=$(uname -s)
case "$os" in
	Linux) goos=linux ;;
	Darwin) goos=darwin ;;
	FreeBSD) goos=freebsd ;;
	OpenBSD | NetBSD | DragonFly)
		die "no prebuilt binaries for $os yet — install with: go install github.com/$REPO/cmd/mdv@latest"
		;;
	*) die "unsupported OS: $os (Windows: download from https://github.com/$REPO/releases)" ;;
esac

arch=$(uname -m)
case "$arch" in
	x86_64 | amd64) goarch=amd64 ;;
	aarch64 | arm64) goarch=arm64 ;;
	*) die "unsupported architecture: $arch" ;;
esac

say "Finding the latest release..."
tag=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
	grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
[ -n "$tag" ] || die "could not determine the latest release"
version=${tag#v}

asset="mdv_${version}_${goos}_${goarch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "Downloading $asset ($tag)..."
curl -fsSL -o "$tmp/$asset" "$base/$asset" ||
	die "download failed — is there a $goos/$goarch build for $tag? Fallback: go install github.com/$REPO/cmd/mdv@latest"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || die "checksum download failed"

say "Verifying checksum..."
expected=$(grep "$asset" "$tmp/checksums.txt" | awk '{print $1}')
[ -n "$expected" ] || die "$asset not found in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
else
	die "need sha256sum or shasum to verify the download"
fi
[ "$expected" = "$actual" ] || die "checksum mismatch for $asset"

tar -xzf "$tmp/$asset" -C "$tmp"
[ -f "$tmp/mdv" ] || die "archive did not contain the mdv binary"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/mdv" "$INSTALL_DIR/mdv" 2>/dev/null ||
	{ cp "$tmp/mdv" "$INSTALL_DIR/mdv" && chmod 0755 "$INSTALL_DIR/mdv"; }
say "Installed mdv $tag to $INSTALL_DIR/mdv"

if [ -f "$tmp/docs/mdv.1" ]; then
	man_dir="${XDG_DATA_HOME:-$HOME/.local/share}/man/man1"
	if mkdir -p "$man_dir" 2>/dev/null && cp "$tmp/docs/mdv.1" "$man_dir/mdv.1" 2>/dev/null; then
		say "Installed man page to $man_dir/mdv.1"
	fi
fi

case ":$PATH:" in
	*":$INSTALL_DIR:"*)
		say "$INSTALL_DIR is already on your PATH."
		;;
	*)
		line="export PATH=\"$INSTALL_DIR:\$PATH\""
		shell_name=$(basename "${SHELL:-/bin/sh}")
		case "$shell_name" in
			zsh) rc="$HOME/.zshrc" ;;
			bash) rc="$HOME/.bashrc" ;;
			*) rc="$HOME/.profile" ;;
		esac
		if [ -f "$rc" ] && grep -F "$line" "$rc" >/dev/null 2>&1; then
			say "$rc already adds $INSTALL_DIR to PATH."
		else
			printf '\n# Added by the mdv installer\n%s\n' "$line" >>"$rc"
			say "Added $INSTALL_DIR to PATH in $rc"
		fi
		say "Open a new shell (or run: . $rc) so 'mdv' is found."
		;;
esac

say ""
say "Done. Try:  mdv --version"
