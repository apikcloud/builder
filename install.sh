#!/bin/sh
# Install the latest odoo-builder release for this platform.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/apikcloud/odoo-builder/main/install.sh | sh
#
# Env vars:
#   VERSION      release tag to install, e.g. "v1.2.3" (default: latest)
#   INSTALL_DIR  directory to install the binary into (default: /usr/local/bin,
#                falls back to $HOME/.local/bin if not writable)

set -eu

REPO="apikcloud/odoo-builder"
BIN_NAME="odoo-builder"

log() { printf '%s\n' "$*" >&2; }
die() { log "error: $*"; exit 1; }

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "required command '$1' not found"
}

need_cmd curl
need_cmd tar

os=$(uname -s)
case "$os" in
	Linux) os="linux" ;;
	*) die "unsupported OS: $os (only linux binaries are published)" ;;
esac

arch=$(uname -m)
case "$arch" in
	x86_64|amd64) arch="amd64" ;;
	aarch64|arm64) arch="arm64" ;;
	*) die "unsupported architecture: $arch" ;;
esac

version="${VERSION:-}"
if [ -z "$version" ]; then
	version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
		grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
	[ -n "$version" ] || die "could not determine latest release version"
fi

asset="${BIN_NAME}-${os}-${arch}.tar.gz"
base_url="https://github.com/${REPO}/releases/download/${version}"

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

log "downloading ${asset} (${version})..."
curl -fsSL -o "${tmp_dir}/${asset}" "${base_url}/${asset}" ||
	die "failed to download ${base_url}/${asset}"

curl -fsSL -o "${tmp_dir}/checksums.txt" "${base_url}/checksums.txt" ||
	die "failed to download checksums.txt"

log "verifying checksum..."
(
	cd "$tmp_dir"
	grep " ${asset}\$" checksums.txt > checksum.expected ||
		die "no checksum entry for ${asset}"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum -c checksum.expected
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 -c checksum.expected
	else
		die "no sha256sum/shasum available to verify download"
	fi
)

tar -xzf "${tmp_dir}/${asset}" -C "$tmp_dir"

install_dir="${INSTALL_DIR:-/usr/local/bin}"
if [ ! -w "$install_dir" ] 2>/dev/null; then
	install_dir="${HOME}/.local/bin"
	mkdir -p "$install_dir"
fi

install_path="${install_dir}/${BIN_NAME}"
mv "${tmp_dir}/${BIN_NAME}-${os}-${arch}" "$install_path"
chmod +x "$install_path"

log "installed ${BIN_NAME} ${version} to ${install_path}"

case ":$PATH:" in
	*":${install_dir}:"*) ;;
	*) log "warning: ${install_dir} is not on your PATH" ;;
esac

"$install_path" version 2>/dev/null || true
