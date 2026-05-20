# agent-go-docker

用于启动 Claude Code 容器环境的 Docker 镜像、本地启动脚本（`agent-go`）以及 HTTP runner 服务（`runner/`）。

## 目录结构

- `Dockerfile` / `Dockerfile.<variant>`：基础镜像与各语言变体镜像。
- `agent-go`：本地启动脚本，负责安装 `agent-cc` / `agent-cc-web` / `agent-cc-tmux` 等命令。
- `entrypoint.sh`：容器入口，处理 UID 映射、tmux/ttyd 启动等。
- `runner/`：基于 Go 的 HTTP runner，提供按项目动态拉起 agent 容器的 API。
- `build.sh`：一键构建并推送全部多架构镜像（含 runner）。

## 构建说明

### 本地构建基础镜像

```bash
docker build -t agent-go-docker:latest -f Dockerfile .
```

如需走代理拉取依赖，可通过 `--build-arg HTTP_PROXY=...` 传入，构建结束代理变量会被清空：

```bash
docker build --build-arg HTTP_PROXY=http://10.1.2.12:8118 \
  -t agent-go-docker:latest -f Dockerfile .
```

### 构建语言变体镜像

```bash
docker build -t agent-go-docker:java8  -f Dockerfile.java8 .
docker build -t agent-go-docker:java17 -f Dockerfile.java17 .
docker build -t agent-go-docker:java21 -f Dockerfile.java21 .
docker build -t agent-go-docker:java25 -f Dockerfile.java25 .
docker build -t agent-go-docker:go     -f Dockerfile.go .
docker build -t agent-go-docker:rust   -f Dockerfile.rust .
```

### 构建 runner 镜像

`runner/Dockerfile` 同样支持 `HTTP_PROXY` 构建参数，并内置 Docker CLI 以便调用宿主 docker socket：

```bash
cd runner
docker build --build-arg HTTP_PROXY=http://10.1.2.12:8118 \
  -t agent-run:latest .
```

## 使用说明

### 1. 安装启动脚本

给脚本增加可执行权限，并安装命令链接：

```bash
chmod +x agent-go
./agent-go add
export PATH="$HOME/.local/bin:$PATH"
```

安装后可使用以下命令：

- `agent-cc`：启动 Claude Code 交互式 CLI
- `agent-cc-web`：启动 ttyd Web 终端 + tmux
- `agent-cc-tmux`：在 tmux 中启动 Claude Code

### 2. 基本启动

```bash
agent-cc
```

### 3. 选择镜像变体

```bash
agent-cc --java8
agent-cc --java
agent-cc --java21
agent-cc --java25
agent-cc --go
agent-cc --rust
```

其中 `--java` 等同于 `--java17`。

### 4. 传递 Claude 参数

```bash
agent-cc -p '帮我检查当前目录代码'
```

### 5. Web / tmux 模式

```bash
agent-cc-web
agent-cc-tmux
```

### 6. 常用环境变量

```bash
export AGENT_ID=default
export AGENT_IMAGE_REGISTRY=ghcr.io/mark0725/agent-go-docker
export CLAUDE_HOME=$HOME/.claude
export AGENTS_HOME=$HOME/.agents
export AGENTS_HUB=$HOME/.agents-hub
```

### 7. 直接使用 Docker 运行

```bash
docker run -it --rm --network=host \
  --user 0 \
  -e "HOST_UID=$(id -u)" \
  -e "HOST_GID=$(id -g)" \
  -e "HOME=/home/node" \
  -e "AGENT_ID=default" \
  -v node_home:/home/node \
  -v "$PWD:/workspace/$(pwd | sed 's#/#_#g')" \
  -v "$HOME/.claude:/home/node/.claude" \
  -v "$HOME/.agents:/home/node/.agents" \
  -v "$HOME/.agents-hub:/home/node/.agents-hub" \
  -w "/workspace/$(pwd | sed 's#/#_#g')" \
  ghcr.io/mark0725/agent-go-docker:latest \
  claude
```

## Runner 服务

`runner/` 是一个 Go 编写的 HTTP 服务，监听 `:8080`，根据请求动态启动/管理 agent 容器，并按项目分配 `7681-7780` 范围的端口暴露 ttyd Web 终端。

### 启动

推荐让 runner 与 agent 共用 host 网络，反代路径最短：

```bash
docker run -d --name agent-run \
  --network=host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /data/work:/data/work \
  -e "HOST_UID=$(id -u)" \
  -e "HOST_GID=$(id -g)" \
  -e "AGENT_ID=default" \
  -e "CLAUDE_HOME=${HOME}/.claude" \
  -e "CODEX_HOME=${HOME}/.codex" \
  -e "AGENTS_HUB=${HOME}/.agents-hub" \
  -e "AGENTS_HOME=${HOME}/.agents" \
  -e "AGENT_IMAGE_REGISTRY=ghcr.io/mark0725/agent-go-docker" \
  -e "AGENT_IMAGE_TAG=latest" \
  ghcr.io/mark0725/agent-run:latest
```

agent 容器固定以 `--network=host` 启动，ttyd 直接绑在宿主 `7681-7780` 端口，无需 `-p` 映射。

如果 runner 必须运行在 bridge 网络上，需要让它能反代到宿主上的 agent ttyd：

```bash
docker run -d --name agent-run \
  -p 8080:8080 \
  --add-host=host.docker.internal:host-gateway \
  -e "RUNNER_PROXY_HOST=host.docker.internal" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  ...
```

### 鉴权

默认无认证，runner 端口暴露给可信网络即可。设置 `RUNNER_AUTH_TOKEN` 即可启用 token 鉴权：

```bash
-e "RUNNER_AUTH_TOKEN=$(openssl rand -hex 24)"
```

启用后，所有 `/api/*`、`/proxy/*` 和 UI 都会鉴权。客户端可用以下任一方式：

- `Authorization: Bearer <token>` —— 适合 API/curl。
- Cookie `runner_token=<token>` —— 浏览器持久访问。
- URL `?token=<token>` —— 首次匹配成功后，runner 会回写 HttpOnly cookie，后续刷新和 iframe 内 ttyd 反代请求自动带 cookie。

`/health` 始终不鉴权，方便健康检查。

### 常用环境变量

- `LISTEN_ADDR`：HTTP 监听地址，默认 `:8080`。
- `DOCKER_SOCK`：宿主 Docker socket，默认 `/var/run/docker.sock`。
- `AGENT_ID`：注入到 agent 容器的默认 `AGENT_ID`，页面创建表单留空时使用，默认 `default`。
- `HOST_UID` / `HOST_GID`：透传给 agent 容器的宿主 UID/GID，避免 agent 写出的工作区文件归 root；建议设为 `$(id -u)` / `$(id -g)`。
- `AGENT_IMAGE_REGISTRY` / `AGENT_IMAGE_TAG`：agent 镜像与标签。
- `RUNNER_PROXY_HOST`：runner 反代 agent ttyd 时使用的主机名，默认 `127.0.0.1`（要求 runner 与 agent 共享 host 网络）。bridge 网络场景设为 `host.docker.internal` 并配合 `--add-host=host.docker.internal:host-gateway`。
- `RUNNER_AUTH_TOKEN`：访问 runner 页面和 API 的共享 token，默认空（无需认证）。详见上文「鉴权」。
- `PROJECT_ROOT`：项目工作区根目录，默认 `/data/work`。
- `PROJECT_HOME`：覆盖 `${PROJECT_ROOT}/${PROJECT_ID}`，强制使用同一个工作区目录。
- `CLAUDE_HOME` / `CODEX_HOME` / `AGENTS_HOME` / `AGENTS_HUB`：宿主侧目录，挂载到 agent 容器的 `/home/node/.{claude,codex,agents,agents-hub}`。默认基于 runner 用户 `$HOME`。
- `PORT_RANGE_START` / `PORT_RANGE_END`：可分配端口区间，默认 `7681-7780`。
- `ANTHROPIC_AUTH_TOKEN`、`ANTHROPIC_BASE_URL`、`HTTP_PROXY`、`HTTPS_PROXY`、`PROXY_URL`：透传到 agent 容器。
