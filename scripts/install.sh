#!/bin/sh
# Install upc from a GitHub release (macOS and Linux, amd64 and arm64).
#
# Downloads the release archive and checksums.txt, verifies the archive's
# sha256 against checksums.txt, and only then installs the binary. Any failure
# aborts without installing (fail closed). No sudo: the default target is
# $HOME/.local/bin; override with UPC_INSTALL_DIR.
#
# Environment:
#   UPC_VERSION      release tag such as v0.2.0 (default: latest release)
#   UPC_INSTALL_DIR  install directory (default: $HOME/.local/bin)
#   UPC_REPO         owner/name (default: justinfischman/universal-package-core)
set -eu

repo="${UPC_REPO:-justinfischman/universal-package-core}"
base="https://github.com/${repo}/releases"

die() {
	printf 'install.sh: error: %s\n' "$*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "required tool not found: $1"
}

fetch() { # url dest
	curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL -o "$2" "$1" || die "download failed: $1"
}

sha256_of() { # file
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d ' ' -f 1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d ' ' -f 1
	else
		die "need sha256sum or shasum to verify the download"
	fi
}

case "$repo" in
*/*/* | */ | /* | *[!A-Za-z0-9._/-]*) die "invalid UPC_REPO: ${repo}" ;;
*/*) ;;
*) die "invalid UPC_REPO: ${repo}" ;;
esac

need curl
need tar
need uname
need mktemp

case "$(uname -s)" in
Darwin) os=darwin ;;
Linux) os=linux ;;
*) die "unsupported OS: $(uname -s) (macOS and Linux only)" ;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) die "unsupported architecture: $(uname -m)" ;;
esac

tag="${UPC_VERSION:-}"
if [ -z "$tag" ]; then
	final="$(curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fsSL -o /dev/null -w '%{url_effective}' "${base}/latest")" ||
		die "could not resolve the latest release"
	tag="${final##*/}"
fi
case "$tag" in
v[0-9]*) ;;
*) die "invalid release tag: ${tag}" ;;
esac
case "$tag" in
*[!A-Za-z0-9._-]*) die "invalid release tag: ${tag}" ;;
esac

version="${tag#v}"
archive="upc_${version}_${os}_${arch}.tar.gz"

tmp="$(mktemp -d)" || die "could not create a temporary directory"
trap 'rm -rf "$tmp"' EXIT
trap 'exit 1' INT TERM

printf 'Downloading upc %s (%s/%s)\n' "$tag" "$os" "$arch"
fetch "${base}/download/${tag}/${archive}" "${tmp}/${archive}"
fetch "${base}/download/${tag}/checksums.txt" "${tmp}/checksums.txt"

# checksums.txt lines are "<sha256>  <file name>"; require exactly one match.
expected="$(awk -v f="$archive" '$2 == f { print $1 }' "${tmp}/checksums.txt")"
[ -n "$expected" ] || die "${archive} is not listed in checksums.txt"
[ "$(printf '%s\n' "$expected" | wc -l | tr -d ' ')" = 1 ] ||
	die "${archive} is listed more than once in checksums.txt"

actual="$(sha256_of "${tmp}/${archive}")"
[ "$actual" = "$expected" ] ||
	die "checksum mismatch for ${archive} (expected ${expected}, got ${actual}); nothing installed"
printf 'Checksum verified: %s\n' "$actual"

mkdir "${tmp}/extract"
tar -xzf "${tmp}/${archive}" -C "${tmp}/extract" upc || die "archive does not contain upc"
[ ! -L "${tmp}/extract/upc" ] || die "upc in the archive is a symlink; refusing to install"
[ -f "${tmp}/extract/upc" ] || die "archive does not contain upc"

dest="${UPC_INSTALL_DIR:-${HOME:?HOME is not set}/.local/bin}"
mkdir -p "$dest" || die "cannot create ${dest}"
[ -w "$dest" ] || die "${dest} is not writable; set UPC_INSTALL_DIR to a writable directory"
staged="$(mktemp "${dest}/.upc.XXXXXX")" || die "could not create a file in ${dest}"
if ! { cp "${tmp}/extract/upc" "$staged" && chmod 0755 "$staged" && mv -f "$staged" "${dest}/upc"; }; then
	rm -f "$staged"
	die "could not install to ${dest}"
fi

printf 'Installed %s/upc\n' "$dest"
case ":${PATH}:" in
*":${dest}:"*) ;;
*) printf 'Note: %s is not on your PATH.\n' "$dest" ;;
esac
