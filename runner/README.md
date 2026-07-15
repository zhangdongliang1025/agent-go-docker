# Agent Runner 服务

Agent Runner 是一个基于 Go 的 HTTP 服务，提供 Web UI 和 API 来动态管理 Claude Code Agent 容器。

## 功能特性

- **Web 管理界面** - 基于 Alpine.js 的单页应用，支持创建、启动、停止、重启、删除 Agent
- **反向代理** - 所有 ttyd 终端流量通过 Runner 反向代理，无需暴露主机端口
- **会话持久化** - 基于 shpool 的会话持久化，重启容器后会话不丢失
- **多项目支持** - 支持按项目和工作区组织 Agent
- **认证保护** - 可选的 Token 认证（Bearer/Cookie/URL 参数）
- **环境变量注入** - 支持自定义环境变量注入到 Agent 容器

## 快速开始

### 构建 Runner 镜像

```bash
cd runner
docker build -t agent-run:latest .
```

### 启动 Runner 服务

```bash
# 创建共享网络（如果不存在）
docker network create agents-net 2>/dev/null || true

# 启动 Runner
docker run -d --name agent-run \
  --network=agents-net \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /data/work:/data/work \
  -e "RUNNER_AUTH_TOKEN=$(openssl rand -hex 24)" \
  agent-run:latest
```

访问 http://localhost:8080 打开管理界面。

## API 接口

### Agent 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/agents` | 列出所有 Agent |
| POST | `/api/agents` | 创建 Agent |
| GET | `/api/agents/{id}` | 获取 Agent 详情 |
| PUT | `/api/agents/{id}` | 更新并重建 Agent |
| DELETE | `/api/agents/{id}` | 删除 Agent |
| POST | `/api/agents/{id}/start` | 启动 Agent |
| POST | `/api/agents/{id}/stop` | 停止 Agent |
| POST | `/api/agents/{id}/restart` | 重启 Agent |

### 其他接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/config` | 获取 Runner 配置 |
| GET | `/api/lists/agents` | 列出所有 Agent ID |
| GET | `/api/lists/projects` | 列出所有项目 |
| GET | `/api/lists/workspaces` | 列出工作区 |
| GET | `/proxy/{id}/` | 代理到 Agent 的 ttyd 终端 |
| GET | `/health` | 健康检查（无需认证） |

## 认证

当设置了 `RUNNER_AUTH_TOKEN` 环境变量时，Runner 会启用认证。支持三种认证方式：

1. **Authorization Header**: `Authorization: Bearer <token>`
2. **Cookie**: `runner_token=<token>`
3. **URL 参数**: `?token=<token>`

### 创建带认证的 Runner

```bash
docker run -d --name agent-run \
  --network=agents-net \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /data/work:/data/work \
  -e "RUNNER_AUTH_TOKEN=your-secret-token" \
  agent-run:latest
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `LISTEN_ADDR` | `:8080` | HTTP 监听地址 |
| `DOCKER_SOCK` | `/var/run/docker.sock` | Docker socket 路径 |
| `AGENT_IMAGE_REGISTRY` | `ghcr.io/mark0725/agent-go-docker` | Agent 镜像仓库 |
| `AGENT_IMAGE_TAG` | `latest` | Agent 镜像标签 |
| `PROJECT_ROOT` | `/data/work` | 项目根目录 |
| `RUNNER_AUTH_TOKEN` | - | 认证 Token（空则不启用） |
| `ANTHROPIC_AUTH_TOKEN` | - | Anthropic API Token |
| `ANTHROPIC_BASE_URL` | - | Anthropic API 基础 URL |
| `AGENT_ID` | `default` | 默认 Agent ID |
| `HOST_UID` / `HOST_GID` | 自动检测 | 主机用户 ID |
| `TZ` | `Asia/Shanghai` | 时区 |

## 架构说明

```
┌─────────────────┐
│   浏览器用户     │
│  (localhost)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌─────────────────┐
│  Runner (8080)  │────▶│  Agent Container │
│   - Web UI      │     │  - ttyd (7681)   │
│   - API         │     │  - shpool        │
│   - Proxy       │     │  - Claude Code   │
└─────────────────┘     └─────────────────┘
         │
         ▼
┌─────────────────┐
│ Docker Socket   │
│ (管理容器)       │
└─────────────────┘
```

## 开发

### 本地运行

```bash
cd runner
go mod tidy
go run .
```

### 构建二进制

```bash
cd runner
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o agent-run .
```
