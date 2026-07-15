# agent-go-docker

统一开发环境 Docker 镜像，内置 Claude Code 和 OpenAI Codex，支持 Java/Go/Rust/Python/Node.js 多语言开发。

## 特性

- **多 JDK 版本**: 8, 11, 17, 21, 25（默认 17，可切换）
- **多语言支持**: Go, Rust, Python 3, Node.js 24
- **AI 工具**: Claude Code + OpenAI Codex
- **开发工具**: git, vim, neovim, ripgrep, fd-find, jq, tree, htop
- **构建工具**: Maven, cargo, pip, npm
- **Web 终端**: 可选 ttyd + shpool 会话持久化
- **无头浏览器**: 内置 Playwright + Chromium（可选）

## 快速开始

### 安装命令

```bash
chmod +x dev-go
./dev-go add
export PATH="$HOME/.local/bin:$PATH"
```

安装后可用命令：
- `dev-cc` - 交互式 CLI 模式
- `dev-cc-web` - Web 终端（后台运行，浏览器访问）
- `dev-cc-tmux` - shpool 会话模式

### 基本用法

```bash
# 当前目录启动
dev-cc

# 指定项目路径
dev-cc -p ~/projects/myapp

# 指定 JDK 版本
dev-cc --java21

# 组合使用
dev-cc -p ~/projects/myapp --java11
```

### 直接运行 Docker

```bash
docker run -it --rm --network=host \
  --user 0 \
  -e "HOST_UID=$(id -u)" \
  -e "HOST_GID=$(id -g)" \
  -e "HOME=/home/node" \
  -e "AGENT_ID=default" \
  -e "JDK_VERSION=17" \
  -v node_home:/home/node \
  -v "$PWD:/data" \
  -v "$HOME/.claude:/home/node/.claude" \
  -v "$HOME/.m2:/home/node/.m2" \
  -w "/data" \
  ghcr.io/mark0725/agent-go-docker:latest \
  claude
```

## 三种启动模式详解

### 1. dev-cc（交互式 CLI）

直接在终端中运行 Claude Code，适合日常开发使用。

```bash
dev-cc                          # 当前目录启动
dev-cc -p ~/projects/myapp    # 指定项目
```

### 2. dev-cc-web（Web 终端）

在后台启动一个 Docker 容器，通过浏览器访问 Web 终端（基于 ttyd）。

```bash
dev-cc-web                    # 启动 Web 终端
dev-cc-web -p ~/projects/myapp  # 指定项目
```

启动后访问：`http://localhost:7681`

**特点：**
- 后台运行，关闭终端不影响
- 基于浏览器的终端，支持多标签页
- 基于 shpool 的会话持久化，刷新页面会话不丢失
- 适合长时间运行或远程访问

### 3. dev-cc-tmux（shpool 会话）

在 Docker 容器内启动 shpool 会话，适合需要会话持久化的场景。

```bash
dev-cc-tmux                   # 启动 shpool 会话
dev-cc-tmux -p ~/projects/myapp # 指定项目
```

**特点：**
- 会话持久化，断开连接后重新连接会话仍在
- 适合网络不稳定的环境
- 支持多会话管理

## 目录结构

```
.
├── Dockerfile          # 多阶段构建（包含所有环境）
├── entrypoint.sh       # 容器入口（UID 映射 + JDK 切换）
├── dev-go              # 启动脚本
├── .dockerignore       # Docker 构建忽略
├── .tmux.conf          # tmux 配置
├── runner/             # HTTP 运行器服务
│   ├── main.go
│   ├── Dockerfile
│   ├── README.md       # Runner 详细文档
│   └── internal/
└── .github/            # GitHub Actions
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `AGENT_ID` | Agent 标识 | `default` |
| `JDK_VERSION` | JDK 版本 (8/11/17/21/25) | `17` |
| `DATA_MOUNT` | 容器内数据挂载点 | `/data` |
| `AGENT_IMAGE_REGISTRY` | 镜像仓库 | `ghcr.io/mark0725/agent-go-docker` |
| `CLAUDE_HOME` | Claude 配置目录 | `~/.claude` |
| `AGENTS_HOME` | Agents 数据目录 | `~/.agents` |
| `AGENTS_HUB` | Agents Hub 目录 | `~/.agents-hub` |
| `ANTHROPIC_AUTH_TOKEN` | Anthropic API Token | - |
| `HTTP_PROXY` | HTTP 代理 | - |
| `HTTPS_PROXY` | HTTPS 代理 | - |

## 构建

```bash
# 本地构建
docker build -t agent-go-docker:latest .

# 带代理构建
docker build --build-arg HTTP_PROXY=http://proxy:8118 -t agent-go-docker:latest .
```

## Runner 服务

HTTP 服务，动态管理 Agent 容器。详见 [runner/README.md](runner/README.md)。

```bash
docker run -d --name agent-run \
  --network=agents-net \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  ghcr.io/mark0725/agent-run:latest
```

### Runner 功能

- **Web 管理界面** - 基于 Alpine.js 的单页应用
- **反向代理** - 所有 ttyd 终端流量通过 Runner 代理
- **会话持久化** - 基于 shpool 的会话持久化
- **多项目支持** - 按项目和工作区组织 Agent
- **认证保护** - 可选的 Token 认证

### Runner 启动（带认证）

```bash
docker run -d --name agent-run \
  --network=agents-net \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /data/work:/data/work \
  -e "RUNNER_AUTH_TOKEN=$(openssl rand -hex 24)" \
  ghcr.io/mark0725/agent-run:latest
```

## 无头浏览器支持

镜像内置了 Playwright 和 Chromium 浏览器，支持无头浏览器自动化。

### 使用 Playwright

```bash
# 在容器中运行 Playwright 脚本
docker run -it --rm \
  --network=host \
  -v "$PWD:/data" \
  -w "/data" \
  ghcr.io/mark0725/agent-go-docker:latest \
  bash -c "python3 -m playwright install chromium && python3 your_script.py"
```

### 示例脚本

```python
from playwright.sync_api import sync_playwright

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    page = browser.new_page()
    page.goto("https://example.com")
    print(page.title())
    browser.close()
```

## 许可证

MIT
