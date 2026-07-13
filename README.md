# easy-proxy-cli (`ezp`)

跨平台临时代理环境变量工具。一键开启/关闭 `http_proxy` / `https_proxy` / `all_proxy`，并附带 Node.js 等语言语法糖。

**AI agent 优先，同时对人类友好。**

## 安装

### 一键安装（推荐）

macOS / Linux：

```bash
curl -fsSL https://raw.githubusercontent.com/drswith/easy-proxy-cli/main/scripts/install.sh | bash
```

Windows（PowerShell）：

```powershell
irm https://raw.githubusercontent.com/drswith/easy-proxy-cli/main/scripts/install.ps1 | iex
```

安装脚本会：放入 PATH → 写入默认配置 → **自动探测并写入 shell hook**（无需再手敲 `eval`）。

常用选项：

```bash
# 不改 rc，只装二进制 + 配置
curl -fsSL .../install.sh | bash -s -- --no-modify-rc

# 只写入指定 shell
curl -fsSL .../install.sh | bash -s -- --shell zsh --shell bash
```

### 从源码

```bash
make install   # → $(go env GOPATH)/bin/ezp 或 GOBIN
make build     # → ./bin/ezp
```

## 30 秒上手

```bash
# 安装脚本已跑过 setup 时，新开终端后直接：
ezp on
ezp status
ezp off

# 或手动 setup（探测 $SHELL / 已有 rc，幂等写入）
ezp setup
ezp setup --explain          # 查看探测策略
ezp setup --shell zsh
ezp setup --no-modify-rc     # 只写配置
ezp setup --uninstall        # 移除 hook 块

# 不装 hook 时：
eval "$(ezp on --emit)"
eval "$(ezp off --emit)"

# 只影响一条命令（最适合 agent）：
ezp exec -- curl -I https://www.google.com
```

## Shell 探测策略（自研）

不依赖第三方 shell 库。顺序：

1. `$SHELL`，为空则 `getent` / macOS `dscl` 查账户登录 shell（**不**把 `curl|bash` 当成你的日常 shell）
2. 已存在的 `.zshrc` / `.bashrc` / `.bash_profile` / `.profile` / fish / nushell / PowerShell profile
3. Windows 额外写入 PowerShell profile；Unix 仅在 profile 已存在时同步
4. `--shell` 强制指定；`--no-modify-rc` 跳过

支持 hook：`bash` · `zsh` · `sh` · `fish` · `powershell` · `nu`（cmd 请用 `ezp exec`）

## 为什么需要 hook / eval？

子进程**无法**修改父 shell 的环境变量。因此：

| 方式 | 适用 |
|------|------|
| `ezp setup` 后 `ezp on` | 人类日常（安装脚本默认做） |
| `eval "$(ezp on --emit)"` | 临时 / 脚本 |
| `ezp exec -- <cmd>` | AI agent / CI（推荐） |

## 配置

路径：`~/.easy-proxy/config.toml`（可用 `EASY_PROXY_HOME` 覆盖目录）

```toml
version = 1
default_profile = "default"

[profiles.default]
http = "http://127.0.0.1:7897"
https = "http://127.0.0.1:7897"
socks = "socks5://127.0.0.1:7897"
no_proxy = "localhost,127.0.0.1,::1,.local"

[extras]
mirror_uppercase = true      # 同时设置 HTTP_PROXY 等大写
node_use_env_proxy = true    # NODE_USE_ENV_PROXY=1
# node_extra_ca_certs = "/path/to/corp-ca.pem"
```

### CLI 覆写（优先于配置文件）

```bash
ezp on --emit --host 127.0.0.1 --port 7890
ezp on --emit --http http://127.0.0.1:7897 --socks socks5://127.0.0.1:7897
ezp on --emit --profile company --mode http
ezp on --emit --no-node --no-uppercase
```

## 命令一览

| 命令 | 说明 |
|------|------|
| `ezp on` / `off` | 输出 export/unset（配合 emit/hook） |
| `ezp status` | 查看当前进程中的代理变量 |
| `ezp env` | 打印解析后的环境变量 |
| `ezp exec -- …` | 带代理运行子命令 |
| `ezp doctor` | 探测代理端口是否可达 |
| `ezp setup` | 写配置 + 探测并安装 shell hook |
| `ezp config …` | 读写配置 |
| `ezp profiles` | 列出 profile |
| `ezp hook <shell>` | 打印 hook 脚本（高级） |
| `ezp schema` | Agent 用机器可读 schema |
| `ezp completion …` | shell 补全 |

## Agent 约定

```bash
ezp schema                 # 先读能力与退出码
ezp status --json
ezp env --format=json
ezp doctor --json
ezp exec -- curl -sI https://example.com
```

- stdout：脚本 / JSON / 结果
- stderr：人类提示（`--json` / `--quiet` 时关闭）
- 退出码：`0` ok · `1` error · `2` misconfig · `3` unavailable

详见 [AGENTS.md](./AGENTS.md)。

## License

MIT
