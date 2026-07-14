#!/usr/bin/env bash
# easy-proxy-switch-cli uninstaller for macOS / Linux
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/drswith/easy-proxy-switch-cli/main/scripts/uninstall.sh | bash
#   curl -fsSL ... | bash -s -- --purge
#   curl -fsSL ... | bash -s -- --no-modify-rc
#
# Env:
#   EPS_INSTALL_DIR   binary dir (default: ~/.local/bin)
#   EPS_HOME          config dir override (for --purge)

set -euo pipefail

INSTALL_DIR="${EPS_INSTALL_DIR:-${HOME}/.local/bin}"
PURGE=0
NO_MODIFY_RC=0
SETUP_SHELLS=()
BIN_NAME="eps"

log()  { printf '+ %s\n' "$*" >&2; }
warn() { printf '! %s\n' "$*" >&2; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
uninstall.sh — remove eps (macOS/Linux)

Options:
  --dir DIR           install directory (default: ~/.local/bin)
  --shell NAME        pass to eps setup --uninstall (repeatable): zsh|bash|fish|sh|powershell|nu
  --no-modify-rc      remove binary (+ config with --purge) only; do not edit shell rc files
  --purge             also remove ~/.easy-proxy-switch (or EPS_HOME)
  --help              show this help
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --dir) INSTALL_DIR="$2"; shift 2 ;;
    --shell) SETUP_SHELLS+=("$2"); shift 2 ;;
    --no-modify-rc) NO_MODIFY_RC=1; shift ;;
    --purge) PURGE=1; shift ;;
    --help|-h) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

find_eps() {
  local candidate="${INSTALL_DIR}/${BIN_NAME}"
  if [ -x "$candidate" ]; then
    printf '%s\n' "$candidate"
    return 0
  fi
  if command -v "$BIN_NAME" >/dev/null 2>&1; then
    command -v "$BIN_NAME"
    return 0
  fi
  return 1
}

run_setup_uninstall() {
  local bin="$1"
  local args=(setup --uninstall --bin "$bin")
  local s
  for s in "${SETUP_SHELLS[@]+"${SETUP_SHELLS[@]}"}"; do
    args+=(--shell "$s")
  done
  log "running: $bin ${args[*]}"
  "$bin" "${args[@]}"
}

remove_binary() {
  local path="${INSTALL_DIR}/${BIN_NAME}"
  if [ -f "$path" ]; then
    rm -f "$path"
    log "removed ${path}"
  else
    warn "binary not found at ${path}"
  fi
}

config_dir() {
  if [ -n "${EPS_HOME:-}" ]; then
    case "$EPS_HOME" in
      "~/"*) printf '%s\n' "${HOME}/${EPS_HOME#~/}" ;;
      *) printf '%s\n' "$EPS_HOME" ;;
    esac
  else
    printf '%s\n' "${HOME}/.easy-proxy-switch"
  fi
}

purge_config() {
  local dir
  dir="$(config_dir)"
  if [ -d "$dir" ]; then
    rm -rf "$dir"
    log "removed config ${dir}"
  else
    warn "config directory not found at ${dir}"
  fi
}

main() {
  local bin=""
  if [ "$NO_MODIFY_RC" != "1" ]; then
    if bin="$(find_eps)"; then
      run_setup_uninstall "$bin"
    else
      warn "eps not found; skipping shell hook removal (reinstall eps or run: eps setup --uninstall)"
    fi
  fi
  remove_binary
  if [ "$PURGE" = "1" ]; then
    purge_config
  fi
  cat <<EOF >&2

Done. If hooks were removed, open a new terminal (or source your shell rc).
Config is kept unless you passed --purge.
EOF
}

main
