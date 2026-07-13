#!/usr/bin/env bash
# easy-proxy-cli installer for macOS / Linux
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/drswith/easy-proxy-cli/main/scripts/install.sh | bash
#   curl -fsSL ... | bash -s -- --no-modify-rc
#   curl -fsSL ... | bash -s -- --shell zsh
#
# Env:
#   EZP_VERSION       release tag (default: latest)
#   EZP_INSTALL_DIR   binary dir (default: ~/.local/bin)
#   EZP_REPO          owner/repo (default: drswith/easy-proxy-cli)
#   EZP_BIN_URL       override download URL for the binary archive/file
#   EZP_NO_MODIFY_RC  if set to 1, skip shell hook install

set -euo pipefail

REPO="${EZP_REPO:-drswith/easy-proxy-cli}"
INSTALL_DIR="${EZP_INSTALL_DIR:-${HOME}/.local/bin}"
VERSION="${EZP_VERSION:-latest}"
NO_MODIFY_RC="${EZP_NO_MODIFY_RC:-0}"
SETUP_SHELLS=()
BIN_NAME="ezp"

log()  { printf '+ %s\n' "$*" >&2; }
warn() { printf '! %s\n' "$*" >&2; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
install.sh — install ezp (macOS/Linux)

Options:
  --dir DIR           install directory (default: ~/.local/bin)
  --version TAG       release tag or "latest"
  --shell NAME        pass to ezp setup (repeatable): zsh|bash|fish|sh|powershell|nu
  --no-modify-rc      install binary + config only; do not edit shell rc files
  --help              show this help
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --dir) INSTALL_DIR="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --shell) SETUP_SHELLS+=("$2"); shift 2 ;;
    --no-modify-rc) NO_MODIFY_RC=1; shift ;;
    --help|-h) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

detect_os() {
  local u
  u="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$u" in
    linux*) echo linux ;;
    darwin*) echo darwin ;;
    mingw*|msys*|cygwin*) die "use scripts/install.ps1 on Windows" ;;
    *) die "unsupported OS: $u" ;;
  esac
}

detect_arch() {
  local m
  m="$(uname -m)"
  case "$m" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    armv7l) echo arm ;;
    *) die "unsupported arch: $m" ;;
  esac
}

download() {
  local url="$1" out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url"
  else
    die "need curl or wget"
  fi
}

resolve_latest_tag() {
  need_cmd curl
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n1
}

install_from_release() {
  local os arch tag base url tmp asset
  os="$(detect_os)"
  arch="$(detect_arch)"
  tag="$VERSION"
  if [ "$tag" = "latest" ]; then
    tag="$(resolve_latest_tag || true)"
    [ -n "$tag" ] || return 1
  fi
  # Asset naming matches Makefile release targets: ezp-darwin-arm64, etc.
  asset="${BIN_NAME}-${os}-${arch}"
  base="https://github.com/${REPO}/releases/download/${tag}"
  if [ -n "${EZP_BIN_URL:-}" ]; then
    url="$EZP_BIN_URL"
  else
    url="${base}/${asset}"
  fi

  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  log "downloading ${url}"
  if ! download "$url" "${tmp}/${BIN_NAME}"; then
    # try .tar.gz wrapper
    if download "${url}.tar.gz" "${tmp}/ezp.tgz"; then
      tar -xzf "${tmp}/ezp.tgz" -C "$tmp"
    else
      return 1
    fi
  fi
  mkdir -p "$INSTALL_DIR"
  install -m 0755 "${tmp}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
  log "installed ${INSTALL_DIR}/${BIN_NAME}"
}

install_with_go() {
  need_cmd go
  log "installing via go install github.com/${REPO}/cmd/ezp@${VERSION}"
  local ver="$VERSION"
  [ "$ver" = "latest" ] && ver="latest"
  GOBIN="$INSTALL_DIR" go install "github.com/${REPO}/cmd/ezp@${ver}"
  log "installed ${INSTALL_DIR}/${BIN_NAME}"
}

ensure_path_hint() {
  case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      warn "${INSTALL_DIR} is not on PATH"
      warn "add: export PATH=\"${INSTALL_DIR}:\$PATH\""
      ;;
  esac
}

run_setup() {
  local bin="${INSTALL_DIR}/${BIN_NAME}"
  [ -x "$bin" ] || die "binary not found at $bin"
  local args=(setup --bin "$bin")
  if [ "$NO_MODIFY_RC" = "1" ]; then
    args+=(--no-modify-rc)
  fi
  local s
  for s in "${SETUP_SHELLS[@]+"${SETUP_SHELLS[@]}"}"; do
    args+=(--shell "$s")
  done
  log "running: $bin ${args[*]}"
  "$bin" "${args[@]}"
}

main() {
  mkdir -p "$INSTALL_DIR"
  if [ -n "${EZP_BIN_URL:-}" ]; then
    install_from_release || die "download failed"
  elif install_from_release; then
    :
  elif install_with_go; then
    warn "no GitHub release asset found; used go install fallback"
  else
    die "install failed (no release asset and go install unavailable). Build from source: make install"
  fi
  ensure_path_hint
  # Prefer installed binary on PATH for subsequent calls
  export PATH="${INSTALL_DIR}:$PATH"
  run_setup
  cat <<EOF >&2

Done. Next:
  1) open a new terminal (or source your shell rc)
  2) ezp on
  3) ezp status

Agent tip: ezp exec -- curl -I https://example.com
EOF
}

main
