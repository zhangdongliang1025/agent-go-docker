package config

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

type Config struct {
	ListenAddr   string
	DockerSocket string

	AgentImage    string
	ImageRegistry string
	ImageTag      string

	// HostHome is the host user's $HOME and is used to derive defaults that
	// mirror the agent-go shell script:
	//   CLAUDE_HOME = ${HostHome}/.claude
	//   CODEX_HOME  = ${HostHome}/.codex
	//   AGENTS_HOME = ${HostHome}/.agents
	//   AGENTS_HUB  = ${HostHome}/.agents-hub
	HostHome string

	// ProjectRoot is the host directory that hosts per-project workspaces.
	// Each agent mounts ${ProjectRoot}/${ProjectID} into the container as
	// /workspace/${ProjectID}.
	ProjectRoot string
	// ProjectHome, if set, overrides ProjectRoot+ProjectID for this runner.
	ProjectHome string

	ClaudeHome string
	CodexHome  string
	AgentsHome string
	AgentsHub  string

	PortRangeStart int
	PortRangeEnd   int

	HostUID int
	HostGID int
	TZ      string

	// AgentID is the default AGENT_ID injected into agent containers when the
	// per-agent override is empty. Sourced from the runner's own AGENT_ID env
	// var (defaults to "default") to mirror the agent-go shell script.
	AgentID string

	// ProxyHost is the hostname used when the runner reverse-proxies to an
	// agent container's ttyd. Defaults to 127.0.0.1, which only works when
	// the runner shares the host network namespace with the agent. Set to
	// e.g. "host.docker.internal" when running the runner on a bridge
	// network with --add-host=host.docker.internal:host-gateway.
	ProxyHost string

	// AuthToken protects the runner's HTTP API and UI. When empty (default)
	// the runner is open. When set, clients must supply the same token via
	// `Authorization: Bearer <token>`, the `runner_token` cookie, or a
	// `?token=<token>` query parameter (which the server promotes to a
	// cookie on first match).
	AuthToken string

	AnthropicAuthToken string
	AnthropicBaseURL   string
	ProxyURL           string
	HTTPProxy          string
	HTTPSProxy         string
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Load() *Config {
	registry := envOrDefault("AGENT_IMAGE_REGISTRY", "ghcr.io/mark0725/agent-go-docker")
	imageTag := envOrDefault("AGENT_IMAGE_TAG", "latest")

	uid, gid := detectUIDGID()

	portStart, _ := strconv.Atoi(envOrDefault("PORT_RANGE_START", "7681"))
	portEnd, _ := strconv.Atoi(envOrDefault("PORT_RANGE_END", "7780"))

	home := homeDir()

	return &Config{
		ListenAddr:    envOrDefault("LISTEN_ADDR", ":8080"),
		DockerSocket:  envOrDefault("DOCKER_SOCK", "/var/run/docker.sock"),
		AgentImage:    fmt.Sprintf("%s:%s", registry, imageTag),
		ImageRegistry: registry,
		ImageTag:      imageTag,

		HostHome:    home,
		ProjectRoot: envOrDefault("PROJECT_ROOT", "/data/work"),
		ProjectHome: os.Getenv("PROJECT_HOME"),

		ClaudeHome: os.Getenv("CLAUDE_HOME"),
		CodexHome:  os.Getenv("CODEX_HOME"),
		AgentsHome: os.Getenv("AGENTS_HOME"),
		AgentsHub:  os.Getenv("AGENTS_HUB"),

		PortRangeStart: portStart,
		PortRangeEnd:   portEnd,

		HostUID: uid,
		HostGID: gid,
		TZ:      envOrDefault("TZ", "Asia/Shanghai"),

		AgentID: envOrDefault("AGENT_ID", "default"),

		ProxyHost: envOrDefault("RUNNER_PROXY_HOST", "127.0.0.1"),

		AuthToken: os.Getenv("RUNNER_AUTH_TOKEN"),

		AnthropicAuthToken: os.Getenv("ANTHROPIC_AUTH_TOKEN"),
		AnthropicBaseURL:   os.Getenv("ANTHROPIC_BASE_URL"),
		ProxyURL:           os.Getenv("PROXY_URL"),
		HTTPProxy:          os.Getenv("HTTP_PROXY"),
		HTTPSProxy:         os.Getenv("HTTPS_PROXY"),
	}
}

func detectUIDGID() (int, int) {
	if v := os.Getenv("HOST_UID"); v != "" {
		uid, _ := strconv.Atoi(v)
		gidStr := os.Getenv("HOST_GID")
		if gidStr == "" {
			gidStr = v
		}
		gid, _ := strconv.Atoi(gidStr)
		return uid, gid
	}
	if u, err := user.Current(); err == nil {
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)
		return uid, gid
	}
	return 1000, 1000
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "/root"
}
