#!/bin/sh
# Install the horton CLI into ~/.local/bin (override with --bin-dir or $HORTON_BIN_DIR).
#
#   curl -fsSL https://codeberg.org/kehvyn/horton-cli/raw/branch/main/install.sh | bash
#   curl -fsSL https://codeberg.org/kehvyn/horton-cli/raw/branch/main/install.sh | bash -s -- --bin-dir /usr/local/bin
#
# This script never edits your shell rc files.
set -eu

REPO="kehvyn/horton-cli"
API="https://codeberg.org/api/v1/repos/$REPO"
DOWNLOAD="https://codeberg.org/$REPO/releases/download"

die() {
	echo "install.sh: $*" >&2
	exit 1
}

BIN_DIR=""
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
	-h | --help)
		echo "usage: install.sh [--bin-dir <path>]"
		echo "env: HORTON_VERSION (e.g. v0.1.0), HORTON_BIN_DIR"
		exit 0
		;;
	*)
		die "unknown argument: $1"
		;;
	esac
done

# 1. Platform detection. We ship Linux binaries only.
os="$(uname -s)"
[ "$os" = "Linux" ] || die "no prebuilt binaries for $os; build from source: https://codeberg.org/$REPO#install"

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
if [ -n "${HORTON_VERSION:-}" ]; then
	tag="$HORTON_VERSION"
else
	tag="$(fetch_stdout "$API/releases/latest" |
		grep -o '"tag_name":[[:space:]]*"[^"]*"' |
		sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/' |
		head -n 1)"
	[ -n "$tag" ] || die "could not determine the latest release; set HORTON_VERSION=vX.Y.Z"
fi
version="${tag#v}"

# 4. Resolve the install directory.
if [ -z "$BIN_DIR" ]; then
	BIN_DIR="${HORTON_BIN_DIR:-$HOME/.local/bin}"
fi
mkdir -p "$BIN_DIR" || die "cannot create $BIN_DIR"

archive="horton_${version}_linux_${arch}.tar.gz"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "downloading horton $tag ($arch)"
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
[ -f "$tmp/horton" ] || die "archive did not contain a horton binary"
install -m 0755 "$tmp/horton" "$BIN_DIR/horton" || die "could not install into $BIN_DIR"

echo "installed $("$BIN_DIR/horton" --version) to $BIN_DIR/horton"

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

echo "shell completion: horton completion zsh|bash|fish|powershell"
