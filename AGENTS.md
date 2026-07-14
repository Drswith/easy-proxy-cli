# AGENTS.md — easy-proxy-switch-cli (`eps`)

Instructions for AI coding agents using or extending this CLI.

## Purpose

`eps` toggles temporary proxy-related environment variables for the current shell or a child process. Binary name: **`eps`**.

## Critical constraint

A child process cannot mutate the parent shell environment. Prefer:

1. **`eps exec -- <cmd> [args...]`** — apply proxy only to the child (best for agents)
2. **`eval "$(eps on --emit)"`** — apply to current shell when shell access exists
3. Shell hook via **`eps setup`** (preferred) or `eval "$(eps hook zsh)"` then `eps on` / `eps off`

Never assume bare `eps on` without hook/eval has changed the caller's env.

Installers (`scripts/install.sh`, `scripts/install.ps1`) run `eps setup` automatically unless `--no-modify-rc`.

## Discoverability

```bash
eps schema          # JSON: commands, exit codes, default config
eps --help
eps <cmd> --help
```

## Output conventions

| Stream | Content |
|--------|---------|
| stdout | Shell scripts, JSON (`--json`), command results |
| stderr | Human hints (disabled by `--json` or `--quiet`) |

## Exit codes (stable)

| Code | Meaning |
|------|---------|
| 0 | OK |
| 1 | Generic error |
| 2 | Misconfiguration |
| 3 | Proxy unreachable (`doctor`) |

## Preferred agent workflows

### Inspect

```bash
eps status --json
eps profiles --json
eps config show --json
eps env --format=json
```

### Run one networked command through proxy

```bash
eps exec -- curl -sS -I https://example.com
eps exec --port 7897 -- npm ping
```

### Enable in shell session (when you control the shell)

```bash
eval "$(eps on --emit --quiet)"
# ... work ...
eval "$(eps off --emit --quiet)"
```

### Health check

```bash
eps doctor --json
# exit 3 => proxy down
```

## Config

- Path: `~/.easy-proxy-switch/config.toml` (override dir with `EPS_HOME`)
- Default profile listens on `127.0.0.1:7897` (Clash/V2Ray mixed port convention)
- CLI flags override config fields

### Env vars set by default `on`

- `http_proxy`, `https_proxy`, `all_proxy`, `no_proxy`
- Uppercase mirrors when `mirror_uppercase` (for Go etc.)
- `NODE_USE_ENV_PROXY=1` when `node_use_env_proxy` (Node.js built-in proxy)

## Do / Don't

- **Do** use `--json` for parsing
- **Do** use `eps exec` for one-shot commands
- **Don't** parse human stderr text
- **Don't** expect `eps on` alone to export into the parent process without hook/eval
