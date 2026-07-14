**English** | [中文](README.md)

# easy-proxy-switch-cli (`eps`)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](go.mod)
[![CI](https://img.shields.io/github/actions/workflow/status/drswith/easy-proxy-switch-cli/ci.yml?branch=main&label=CI&logo=github)](https://github.com/drswith/easy-proxy-switch-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/drswith/easy-proxy-switch-cli?include_prereleases&logo=github)](https://github.com/drswith/easy-proxy-switch-cli/releases)
[![Downloads](https://img.shields.io/github/downloads/drswith/easy-proxy-switch-cli/total?label=Downloads&logo=github)](https://github.com/drswith/easy-proxy-switch-cli/releases)
[![Latest Downloads](https://img.shields.io/github/downloads/drswith/easy-proxy-switch-cli/latest/total?label=Latest%20Downloads&logo=github)](https://github.com/drswith/easy-proxy-switch-cli/releases/latest)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#installation)

Cross-platform proxy switch CLI: toggle `http_proxy` / `https_proxy` / `all_proxy` in one step, with extras for Node.js and other runtimes. Configuration only specifies the local proxy port; the core value is shell-level on/off, not proxy rule management.

**Agent-first, human-friendly.**

## Installation

### One-line install (recommended)

macOS / Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/drswith/easy-proxy-switch-cli/main/scripts/install.sh | bash
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/drswith/easy-proxy-switch-cli/main/scripts/install.ps1 | iex
```

The installer: adds to PATH → writes default config → **auto-detects and installs shell hooks** (no manual `eval` needed).

Common options:

```bash
# Skip rc changes; install binary + config only
curl -fsSL .../install.sh | bash -s -- --no-modify-rc

# Target specific shells only
curl -fsSL .../install.sh | bash -s -- --shell zsh --shell bash
```

### From source

```bash
make install   # → $(go env GOPATH)/bin/eps or GOBIN
make build     # → ./bin/eps
```

## 30-second quick start

```bash
# After the installer ran setup, in a new terminal:
eps on
eps status
eps off

# Or run setup manually (detects $SHELL / existing rc files; idempotent)
eps setup
eps setup --explain          # show detection strategy
eps setup --shell zsh
eps setup --no-modify-rc     # config only
eps setup --uninstall        # remove hook blocks

# Without hooks:
eval "$(eps on --emit)"
eval "$(eps off --emit)"

# Proxy a single command only (best for agents):
eps exec -- curl -I https://www.google.com
```

## Shell detection (built-in)

No third-party shell libraries. Detection order:

1. `$SHELL`; if empty, `getent` / macOS `dscl` for the account login shell (**not** the `curl|bash` pipe shell)
2. Existing `.zshrc` / `.bashrc` / `.bash_profile` / `.profile` / fish / nushell / PowerShell profiles
3. On Windows, also writes PowerShell profile; on Unix, syncs only when a profile already exists
4. `--shell` to force a shell; `--no-modify-rc` to skip rc changes

Supported hooks: `bash` · `zsh` · `sh` · `fish` · `powershell` · `nu` (use `eps exec` for cmd)

## Why hooks / eval?

A child process **cannot** mutate the parent shell’s environment. Options:

| Approach | Use case |
|----------|----------|
| `eps setup` then `eps on` | Daily interactive use (installer default) |
| `eval "$(eps on --emit)"` | Ad-hoc sessions / scripts |
| `eps exec -- <cmd>` | AI agents / CI (recommended) |

## Configuration

Path: `~/.easy-proxy-switch/config.toml` (override directory with `EPS_HOME`)

```toml
version = 1
default_profile = "default"

[profiles.default]
http = "http://127.0.0.1:7897"
https = "http://127.0.0.1:7897"
socks = "socks5://127.0.0.1:7897"
no_proxy = "localhost,127.0.0.1,::1,.local"

[extras]
mirror_uppercase = true      # also set HTTP_PROXY etc.
node_use_env_proxy = true    # NODE_USE_ENV_PROXY=1
# node_extra_ca_certs = "/path/to/corp-ca.pem"
```

### CLI overrides (take precedence over config)

```bash
eps on --emit --host 127.0.0.1 --port 7890
eps on --emit --http http://127.0.0.1:7897 --socks socks5://127.0.0.1:7897
eps on --emit --profile company --mode http
eps on --emit --no-node --no-uppercase
```

## Commands

| Command | Description |
|---------|-------------|
| `eps on` / `off` | Print export/unset (with emit/hook) |
| `eps status` | Show proxy vars in the current process |
| `eps env` | Print resolved environment variables |
| `eps exec -- …` | Run a child command through the proxy |
| `eps doctor` | Probe whether the proxy port is reachable |
| `eps setup` | Write config + detect and install shell hooks |
| `eps config …` | Read/write configuration |
| `eps profiles` | List profiles |
| `eps hook <shell>` | Print hook script (advanced) |
| `eps schema` | Machine-readable schema for agents |
| `eps completion …` | Shell completions |

## Agent conventions

```bash
eps schema                 # capabilities and exit codes first
eps status --json
eps env --format=json
eps doctor --json
eps exec -- curl -sI https://example.com
```

- stdout: scripts / JSON / results
- stderr: human hints (disabled with `--json` / `--quiet`)
- Exit codes: `0` ok · `1` error · `2` misconfig · `3` unavailable

See [AGENTS.md](./AGENTS.md) for details.

## Testing

```bash
make test-unit          # internal + cmd
make test-e2e           # build binary and run e2e
make test-all           # vet + unit + e2e

# Docker isolation (requires Docker locally)
make test-docker
# or
./scripts/test-docker.sh unit|e2e|all
```

CI: `.github/workflows/ci.yml` (multi-OS unit, ubuntu e2e, Docker isolation).  
Release: push a `v*` tag to trigger `.github/workflows/release.yml` with per-platform binaries and checksums.

## License

MIT
