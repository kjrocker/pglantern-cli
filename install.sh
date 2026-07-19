#!/bin/sh
# Install the lantern CLI into ~/.local/bin (override with --bin-dir or $LANTERN_BIN_DIR).
#
#   curl -fsSL https://codeberg.org/kehvyn/pglantern-cli/raw/branch/main/install.sh | bash
#   curl -fsSL https://codeberg.org/kehvyn/pglantern-cli/raw/branch/main/install.sh | bash -s -- --bin-dir /usr/local/bin
#   curl -fsSL https://codeberg.org/kehvyn/pglantern-cli/raw/branch/main/install.sh | bash -s -- --bin-name pglantern
#
# This script never edits your shell rc files.
set -eu

REPO="kehvyn/pglantern-cli"
API="https://codeberg.org/api/v1/repos/$REPO"
DOWNLOAD="https://codeberg.org/$REPO/releases/download"

die() {
	echo "install.sh: $*" >&2
	exit 1
}

BIN_DIR=""
BIN_NAME=""
while [ $# -gt 0 ]; do
	case "$1" in
	--bin-dir)
		[ $# -ge 2 ] || die "--bin-dir requires a path"
		BIN_DIR="$2"
		shift 2
		;;
	--bin-dir=*)
		BIN_DIR="${1#--bin-dir=}"
		shift
		;;
	--bin-name)
		[ $# -ge 2 ] || die "--bin-name requires a name"
		BIN_NAME="$2"
		shift 2
		;;
	--bin-name=*)
		BIN_NAME="${1#--bin-name=}"
		shift
		;;
	-h | --help)
		echo "usage: install.sh [--bin-dir <path>] [--bin-name <name>]"
		echo "env: LANTERN_VERSION (e.g. v0.1.0), LANTERN_BIN_DIR, LANTERN_BIN_NAME"
		exit 0
		;;
	*)
		die "unknown argument: $1"
		;;
	esac
done

# 1. Platform detection. We ship Linux and macOS binaries.
case "$(uname -s)" in
Linux) os="linux" ;;
Darwin) os="darwin" ;;
*) die "no prebuilt binaries for $(uname -s); build from source: https://codeberg.org/$REPO#install" ;;
esac

case "$(uname -m)" in
x86_64) arch="amd64" ;;
aarch64 | arm64) arch="arm64" ;;
*) die "unsupported architecture: $(uname -m); build from source: https://codeberg.org/$REPO#install" ;;
esac

# 2. Fetch helpers.
if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL "$1" -o "$2"; }
	fetch_stdout() { curl -fsSL "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -qO "$2" "$1"; }
	fetch_stdout() { wget -qO- "$1"; }
else
	die "neither curl nor wget found"
fi

# 3. Resolve the version.
if [ -n "${LANTERN_VERSION:-}" ]; then
	tag="$LANTERN_VERSION"
else
	tag="$(fetch_stdout "$API/releases/latest" |
		grep -o '"tag_name":[[:space:]]*"[^"]*"' |
		sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/' |
		head -n 1)"
	[ -n "$tag" ] || die "could not determine the latest release; set LANTERN_VERSION=vX.Y.Z"
fi
version="${tag#v}"

# 4. Resolve the install directory.
if [ -z "$BIN_DIR" ]; then
	BIN_DIR="${LANTERN_BIN_DIR:-$HOME/.local/bin}"
fi
mkdir -p "$BIN_DIR" || die "cannot create $BIN_DIR"

# The installed filename is configurable; use --bin-dir to choose the directory.
if [ -z "$BIN_NAME" ]; then
	BIN_NAME="${LANTERN_BIN_NAME:-lantern}"
fi
case "$BIN_NAME" in
*/* | . | ..) die "--bin-name must be a plain filename, not a path: $BIN_NAME" ;;
"") die "--bin-name must not be empty" ;;
esac

archive="lantern_${version}_${os}_${arch}.tar.gz"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "downloading lantern $tag ($arch)"
fetch "$DOWNLOAD/$tag/$archive" "$tmp/$archive" ||
	die "download failed: $DOWNLOAD/$tag/$archive"

# 5. Verify the checksum. A missing sha256 tool warns; a mismatch is fatal.
have_sha=yes
if command -v sha256sum >/dev/null 2>&1; then
	sha_check() { (cd "$tmp" && sha256sum -c -) >/dev/null 2>&1; }
elif command -v shasum >/dev/null 2>&1; then
	sha_check() { (cd "$tmp" && shasum -a 256 -c -) >/dev/null 2>&1; }
else
	have_sha=no
fi

if [ "$have_sha" = no ]; then
	echo "warning: no sha256 tool found; skipping checksum verification" >&2
elif ! fetch "$DOWNLOAD/$tag/checksums.txt" "$tmp/checksums.txt"; then
	echo "warning: could not download checksums.txt; skipping verification" >&2
else
	expected="$(grep " \{1,2\}$archive\$" "$tmp/checksums.txt" || true)"
	if [ -z "$expected" ]; then
		echo "warning: $archive not listed in checksums.txt; skipping verification" >&2
	elif echo "$expected" | sha_check; then
		echo "checksum ok"
	else
		die "checksum verification failed for $archive"
	fi
fi

# 6. Install.
tar -xzf "$tmp/$archive" -C "$tmp" || die "could not extract $archive"
[ -f "$tmp/lantern" ] || die "archive did not contain a lantern binary"
install -m 0755 "$tmp/lantern" "$BIN_DIR/$BIN_NAME" || die "could not install into $BIN_DIR"

echo "installed $("$BIN_DIR/$BIN_NAME" --version) to $BIN_DIR/$BIN_NAME"

# 7. PATH check — we report, we do not edit rc files.
case ":$PATH:" in
*":$BIN_DIR:"*) ;;
*)
	echo >&2
	echo "warning: $BIN_DIR is not on your PATH." >&2
	echo "Add this line to your shell rc file (~/.bashrc, ~/.zshrc, …):" >&2
	echo >&2
	echo "    export PATH=\"$BIN_DIR:\$PATH\"" >&2
	echo >&2
	;;
esac

echo "shell completion: $BIN_NAME completion zsh|bash|fish|powershell"
