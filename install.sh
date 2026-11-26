#!/usr/bin/env sh
# Installer for hos-code: downloads a release archive, checks its sha256, installs the binary and its licences.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Harinder441/hos-code/master/install.sh | sh
#
# Environment variables:
#   VERSION               - version to install (e.g. "0.1.16" or "latest", default: "latest")
#   HOS_CODE_INSTALL_DIR  - target directory for the binary (default: ~/.local/bin, ~/bin or /usr/local/bin)
#   HOS_CODE_REPO         - GitHub repository (default: Harinder441/hos-code)
#   PX0_REPO, PX0_INSTALL_DIR, INSTALL_DIR - older names, still read

set -eu

REPO="${HOS_CODE_REPO:-${PX0_REPO:-Harinder441/hos-code}}"
VERSION="${VERSION:-latest}"

BOLD="\033[1m"
GREEN="\033[38;5;71m"
AMBER="\033[38;5;208m"
RED="\033[38;5;167m"
RESET="\033[0m"

log_info() { printf " ${GREEN}✓${RESET} %b\n" "$1"; }
log_step() { printf " ${AMBER}›${RESET} %b\n" "$1"; }
log_warn() { printf " ${AMBER}!${RESET} %b\n" "$1"; }
log_error() { printf " ${RED}✗${RESET} %b\n" "$1" >&2; }

[ -n "${PX0_REPO:-}" ] && [ -z "${HOS_CODE_REPO:-}" ] && log_warn "PX0_REPO is deprecated; use HOS_CODE_REPO"
[ -n "${PX0_INSTALL_DIR:-}" ] && [ -z "${HOS_CODE_INSTALL_DIR:-}" ] && log_warn "PX0_INSTALL_DIR is deprecated; use HOS_CODE_INSTALL_DIR"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux*)   OS="linux" ;;
  darwin*)  OS="darwin" ;;
  freebsd*) OS="freebsd" ;;
  openbsd*) OS="openbsd" ;;
  netbsd*)  OS="netbsd" ;;
  msys*|cygwin*|mingw*) OS="windows" ;;
  *) log_error "Unsupported operating system: $OS"; exit 1 ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)      ARCH="amd64" ;;
  aarch64|arm64)     ARCH="arm64" ;;
  armv7*|armv6*|arm) ARCH="arm" ;;
  i386|i686)         ARCH="386" ;;
  riscv64)           ARCH="riscv64" ;;
  *) log_error "Unsupported machine architecture: $ARCH"; exit 1 ;;
esac

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --connect-timeout 10 --max-time 30 --retry 2 "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- -T 15 -t 3 "$1"
  else
    log_error "Neither curl nor wget found in PATH. Please install one of them."
    exit 1
  fi
}

fetch_file() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --connect-timeout 10 --max-time 300 --retry 2 -o "$2" "$1"
  else
    wget -q -T 30 -t 3 -O "$2" "$1"
  fi
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}

log_step "Detecting latest release for ${BOLD}${REPO}${RESET} (${OS}/${ARCH})..."

if [ "$VERSION" = "latest" ]; then
  RELEASE_JSON=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)
  VERSION=$(printf '%s' "$RELEASE_JSON" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' | sed 's/^v//')
fi
if [ -z "$VERSION" ] || [ "$VERSION" = "latest" ]; then
  log_error "No published release found for ${REPO}. Specify VERSION=x.y.z or build from source (make build)."
  exit 1
fi

BINARY_EXT=""
ARCHIVE_EXT="tar.gz"
if [ "$OS" = "windows" ]; then BINARY_EXT=".exe"; ARCHIVE_EXT="zip"; fi

USE_SUDO=0
TARGET_DIR=""

is_writable() {
  d="$1"
  [ -z "$d" ] && return 1
  mkdir -p "$d" 2>/dev/null || return 1
  test_file="$d/.hos_code_test_$$"
  if ( : > "$test_file" ) 2>/dev/null; then
    rm -f "$test_file" 2>/dev/null
    return 0
  fi
  return 1
}

for d in "${HOS_CODE_INSTALL_DIR:-}" "${PX0_INSTALL_DIR:-}" "${INSTALL_DIR:-}"; do
  if [ -n "$d" ] && is_writable "$d"; then TARGET_DIR="$d"; break; fi
done

if [ -z "$TARGET_DIR" ]; then
  for d in "${HOME:-}/.local/bin" "${HOME:-}/bin" "/usr/local/bin"; do
    case ":${PATH:-}:" in
      *":$d:"*) if is_writable "$d"; then TARGET_DIR="$d"; break; fi ;;
    esac
  done
fi

if [ -z "$TARGET_DIR" ]; then
  if [ -n "${HOME:-}" ] && is_writable "${HOME}/.local/bin"; then
    TARGET_DIR="${HOME}/.local/bin"
  elif [ -n "${HOME:-}" ] && is_writable "${HOME}/bin"; then
    TARGET_DIR="${HOME}/bin"
  elif is_writable "/usr/local/bin"; then
    TARGET_DIR="/usr/local/bin"
  elif command -v sudo >/dev/null 2>&1 && [ -d "/usr/local/bin" ]; then
    USE_SUDO=1
    TARGET_DIR="/usr/local/bin"
  else
    log_error "No writable install directory; set HOS_CODE_INSTALL_DIR."
    exit 1
  fi
fi

TARGET_BIN="${TARGET_DIR}/hos-code${BINARY_EXT}"
NAME="hos-code-${VERSION}-${OS}-${ARCH}"
ARCHIVE="${NAME}.${ARCHIVE_EXT}"
BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"

TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'hoscodeinstall')"
trap 'rm -rf "$TMP_DIR"' EXIT

log_step "Downloading ${ARCHIVE}..."
if ! fetch_file "${BASE_URL}/${ARCHIVE}" "${TMP_DIR}/${ARCHIVE}" 2>/dev/null; then
  log_error "Failed to download ${BASE_URL}/${ARCHIVE}"
  exit 1
fi
fetch_file "${BASE_URL}/checksums.txt" "${TMP_DIR}/checksums.txt" 2>/dev/null || { log_error "Failed to download checksums.txt"; exit 1; }
WANT=$(grep " ${ARCHIVE}\$" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
GOT=$(sha256_of "${TMP_DIR}/${ARCHIVE}")
if [ -z "$WANT" ] || [ "$WANT" != "$GOT" ]; then
  log_error "Checksum mismatch for ${ARCHIVE}"
  exit 1
fi

if [ "$ARCHIVE_EXT" = "zip" ]; then
  (cd "$TMP_DIR" && unzip -q "$ARCHIVE")
else
  tar -C "$TMP_DIR" -xzf "${TMP_DIR}/${ARCHIVE}"
fi

DOC_DIR="${HOME:-$TMP_DIR}/.local/share/doc/hos-code"
log_step "Installing to ${BOLD}${TARGET_BIN}${RESET} (licences in ${DOC_DIR})..."
if [ "$USE_SUDO" = "1" ]; then
  sudo install -m 0755 "${TMP_DIR}/${NAME}/hos-code${BINARY_EXT}" "$TARGET_BIN"
else
  mkdir -p "$TARGET_DIR"
  install -m 0755 "${TMP_DIR}/${NAME}/hos-code${BINARY_EXT}" "$TARGET_BIN"
fi
mkdir -p "$DOC_DIR" && cp "${TMP_DIR}/${NAME}/LICENSE" "${TMP_DIR}/${NAME}/THIRD_PARTY_NOTICES.md" "$DOC_DIR/" 2>/dev/null || true

log_info "hos-code v${VERSION} installed."

case ":$PATH:" in
  *":$TARGET_DIR:"*) ;;
  *)
    log_warn "${TARGET_DIR} is not in your PATH. Add it with:"
    printf "     export PATH=\"%s:\$PATH\"\n\n" "$TARGET_DIR"
    ;;
esac

printf "\nRun %b to read any directory:\n\n" "${BOLD}hos-code${RESET}"
printf "  ${AMBER}hos-code .${RESET}                 # current directory\n"
printf "  ${AMBER}hos-code /path/to/project${RESET}  # any directory\n\n"
