# easy-proxy-cli (`ezp`)

跨平台临时代理环境变量工具。一键开启/关闭 `http_proxy` / `https_proxy` / `all_proxy`，并附带 Node.js 等语言语法糖。

**AI agent 优先，同时对人类友好。**

## 安装

```bash
# 从源码
cd /Users/drs/workspaces/personal/easy-proxy-cli
make install   # 安装到 $(go env GOPATH)/bin/ezp

# 或本地构建
make build     # 产出 ./bin/ezp
```

## 30 秒上手

```bash
# 1) 生成默认配置 ~/.easy-proxy/config.toml
ezp config init

# 2) 安装 shell hook（推荐，只需一次）
echo 'eval "$(ezp hook zsh)"' >> ~/.zshrc && source ~/.zshrc

# 3) 开关代理（hook 后直接生效）
ezp on
ezp status
ezp off

# 不装 hook 时：
eval "$(ezp on --emit)"
eval "$(ezp off --emit)"

# 只影响一条命令（最适合 agent）：
ezp exec -- curl -I https://www.google.com
```

## 为什么需要 hook / eval？

子进程**无法**修改父 shell 的环境变量。因此：

| 方式 | 适用 |
|------|------|
| `eval "$(ezp hook zsh)"` 后 `ezp on` | 人类日常 |
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
| `ezp config …` | 读写配置 |
| `ezp profiles` | 列出 profile |
| `ezp hook <shell>` | 安装 shell 集成 |
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
