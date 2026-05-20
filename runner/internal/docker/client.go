package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mark0725/agent-go-docker/runner/internal/config"
)

// dockerCreatedAtLayout matches the format produced by `docker ps --format '{{.CreatedAt}}'`,
// e.g. "2026-05-20 16:18:01 +0800 CST".
const dockerCreatedAtLayout = "2006-01-02 15:04:05 -0700 MST"

func parseDockerCreatedAt(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(dockerCreatedAtLayout, s); err == nil {
		return t
	}
	return time.Time{}
}

type Agent struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	AgentID         string    `json:"agentId,omitempty"`
	Status          string    `json:"status"`
	TTYDPort        int       `json:"ttydPort"`
	CreatedAt       time.Time `json:"createdAt"`
	Image           string    `json:"image"`
	Variant         string    `json:"variant,omitempty"`
	ProjectID       string    `json:"projectId,omitempty"`
	WorkspaceID     string    `json:"workspaceId,omitempty"`
	ProjectHome     string    `json:"projectHome,omitempty"`
	ClaudeHome      string    `json:"claudeHome,omitempty"`
	CodexHome       string    `json:"codexHome,omitempty"`
	AgentsHome      string    `json:"agentsHome,omitempty"`
	AgentsHub       string    `json:"agentsHub,omitempty"`
	ClaudeArgs      []string  `json:"claudeArgs,omitempty"`
	HasAuthOverride bool      `json:"hasAuthOverride,omitempty"`
	HasBaseOverride bool      `json:"hasBaseOverride,omitempty"`
}

type Manager struct {
	cfg   *config.Config
	ports *PortAllocator
}

func NewManager(cfg *config.Config, ports *PortAllocator) (*Manager, error) {
	return &Manager{cfg: cfg, ports: ports}, nil
}

func (m *Manager) Config() *config.Config { return m.cfg }

func (m *Manager) RecoverState(ctx context.Context) error {
	output, err := m.dockerOutput(ctx, "ps", "-a", "--filter", "label="+LabelManaged+"=true", "--format", "{{.Labels}}")
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}
	if output == "" {
		return nil
	}
	for _, line := range strings.Split(output, "\n") {
		labels := parseLabelString(line)
		if err := m.ports.RecoverFromLabels(labels); err != nil {
			return err
		}
	}
	return nil
}

type CreateAgentOpts struct {
	Name               string   `json:"name"`
	AgentID            string   `json:"agentId,omitempty"`
	Variant            string   `json:"variant,omitempty"`
	Image              string   `json:"image,omitempty"`
	ProjectID          string   `json:"projectId,omitempty"`
	WorkspaceID        string   `json:"workspaceId,omitempty"`
	ProjectHome        string   `json:"projectHome,omitempty"`
	ClaudeHome         string   `json:"claudeHome,omitempty"`
	CodexHome          string   `json:"codexHome,omitempty"`
	AgentsHome         string   `json:"agentsHome,omitempty"`
	AgentsHub          string   `json:"agentsHub,omitempty"`
	AnthropicAuthToken string   `json:"anthropicAuthToken,omitempty"`
	AnthropicBaseURL   string   `json:"anthropicBaseUrl,omitempty"`
	ClaudeArgs         []string `json:"claudeArgs,omitempty"`
}

// resolvedPaths captures the host-side directories the runner will mount into a
// new agent container, after applying CLI overrides and falling back to the
// agent-go default layout.
type resolvedPaths struct {
	ProjectHome string
	ClaudeHome  string
	CodexHome   string
	AgentsHome  string
	AgentsHub   string
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (m *Manager) resolvePaths(opts CreateAgentOpts, agentID string) resolvedPaths {
	_ = agentID
	home := m.cfg.HostHome

	projectHome := firstNonEmpty(opts.ProjectHome, m.cfg.ProjectHome)
	if projectHome == "" {
		projectHome = m.cfg.ProjectRoot + "/" + opts.ProjectID
	}

	return resolvedPaths{
		ProjectHome: projectHome,
		ClaudeHome:  firstNonEmpty(opts.ClaudeHome, m.cfg.ClaudeHome, home+"/.claude"),
		CodexHome:   firstNonEmpty(opts.CodexHome, m.cfg.CodexHome, home+"/.codex"),
		AgentsHome:  firstNonEmpty(opts.AgentsHome, m.cfg.AgentsHome, home+"/.agents"),
		AgentsHub:   firstNonEmpty(opts.AgentsHub, m.cfg.AgentsHub, home+"/.agents-hub"),
	}
}

// resolveImage picks the docker image to use for an agent in priority order:
// explicit Image > Variant tag > runner default.
func (m *Manager) resolveImage(opts CreateAgentOpts) string {
	if opts.Image != "" {
		return opts.Image
	}
	if v := strings.TrimSpace(opts.Variant); v != "" && v != "default" {
		return fmt.Sprintf("%s:%s", m.cfg.ImageRegistry, v)
	}
	return m.cfg.AgentImage
}

func (m *Manager) CreateAgent(ctx context.Context, opts CreateAgentOpts) (*Agent, error) {
	if opts.Name == "" {
		opts.Name = fmt.Sprintf("agent-%d", time.Now().Unix())
	}

	image := m.resolveImage(opts)

	port, err := m.ports.Allocate()
	if err != nil {
		return nil, err
	}

	projectID := opts.ProjectID
	if projectID == "" {
		projectID = sanitizeName(opts.Name)
	}
	opts.ProjectID = projectID

	workspaceID := strings.TrimSpace(opts.WorkspaceID)
	projectMount := "/workspace/" + projectID
	containerWorkspace := projectMount
	if workspaceID != "" {
		containerWorkspace = projectMount + "/" + workspaceID
	}

	paths := m.resolvePaths(opts, opts.Name)

	agentID := strings.TrimSpace(opts.AgentID)
	if agentID == "" {
		agentID = m.cfg.AgentID
	}
	if agentID == "" {
		agentID = opts.Name
	}
	opts.AgentID = agentID

	containerName := fmt.Sprintf("runner-%s-%d", sanitizeName(opts.Name), time.Now().UnixMilli())

	// Compose the claude command with optional extra args (e.g. --resume <id>).
	claudeCmdTokens := []string{
		"claude",
		"--dangerously-skip-permissions",
		"--allowedTools", "Edit,Write,Bash",
	}
	claudeCmdTokens = append(claudeCmdTokens, opts.ClaudeArgs...)
	claudeCmd := joinShellQuoted(claudeCmdTokens)
	wrappedClaudeCmd := "bash -lc " + shellSingleQuote(". /tmp/agent-env.sh && exec "+claudeCmd)

	startupScript := buildStartupScript(wrappedClaudeCmd, containerWorkspace, port)

	authToken := opts.AnthropicAuthToken
	hasAuthOverride := authToken != ""
	if !hasAuthOverride {
		authToken = m.cfg.AnthropicAuthToken
	}
	baseURL := opts.AnthropicBaseURL
	hasBaseOverride := baseURL != ""
	if !hasBaseOverride {
		baseURL = m.cfg.AnthropicBaseURL
	}

	claudeArgsLabel, _ := json.Marshal(opts.ClaudeArgs)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"--user", "0",
		"--network=host",
		"--label", LabelManaged + "=true",
		"--label", LabelName + "=" + opts.Name,
		"--label", LabelAgentID + "=" + agentID,
		"--label", LabelTTYDPort + "=" + fmt.Sprintf("%d", port),
		"--label", LabelVariant + "=" + opts.Variant,
		"--label", LabelProjectID + "=" + projectID,
		"--label", LabelWorkspaceID + "=" + workspaceID,
		"--label", LabelImage + "=" + image,
		"--label", LabelClaudeArgs + "=" + string(claudeArgsLabel),
		"--label", LabelHasAuthOvr + "=" + boolStr(hasAuthOverride),
		"--label", LabelHasBaseOvr + "=" + boolStr(hasBaseOverride),
		"--label", LabelProjectHome + "=" + paths.ProjectHome,
		"--label", LabelClaudeHome + "=" + paths.ClaudeHome,
		"--label", LabelCodexHome + "=" + paths.CodexHome,
		"--label", LabelAgentsHome + "=" + paths.AgentsHome,
		"--label", LabelAgentsHub + "=" + paths.AgentsHub,
		"-e", fmt.Sprintf("HOST_UID=%d", m.cfg.HostUID),
		"-e", fmt.Sprintf("HOST_GID=%d", m.cfg.HostGID),
		"-e", fmt.Sprintf("TZ=%s", m.cfg.TZ),
		"-e", "HOME=/home/node",
		"-e", "XDG_RUNTIME_DIR=/tmp/runtime-node",
		"-e", "AGENT_ID=" + agentID,
		"-e", "PROJECT_ID=" + projectID,
		"-e", "WORKSPACE_ID=" + workspaceID,
		"-e", "AGENT_WORKSPACE=" + containerWorkspace,
		"-e", "PROJECT_HOME=" + projectMount,
		"-e", "CLAUDE_HOME=/home/node/.claude",
		"-e", "CODEX_HOME=/home/node/.codex",
		"-e", "AGENTS_HOME=/home/node/.agents",
		"-e", "AGENTS_HUB=/home/node/.agents-hub",
	}

	for _, e := range []struct{ k, v string }{
		{"ANTHROPIC_AUTH_TOKEN", authToken},
		{"ANTHROPIC_BASE_URL", baseURL},
		{"PROXY_URL", m.cfg.ProxyURL},
		{"HTTP_PROXY", m.cfg.HTTPProxy},
		{"HTTPS_PROXY", m.cfg.HTTPSProxy},
	} {
		if e.v != "" {
			args = append(args, "-e", fmt.Sprintf("%s=%s", e.k, e.v))
		}
	}

	args = append(args,
		"-v", "node_home_"+sanitizeName(opts.Name)+":/home/node",
		"-v", "/etc/localtime:/etc/localtime:ro",
		"-v", m.cfg.DockerSocket+":/var/run/docker.sock",
		"-v", paths.ProjectHome+":"+projectMount,
		"-v", paths.ClaudeHome+":/home/node/.claude",
		"-v", paths.CodexHome+":/home/node/.codex",
		"-v", paths.AgentsHome+":/home/node/.agents",
		"-v", paths.AgentsHub+":/home/node/.agents-hub",
		"-w", containerWorkspace,
		image,
		"bash", "-lc", startupScript,
	)

	containerID, err := m.dockerOutput(ctx, args...)
	if err != nil {
		m.ports.Release(port)
		return nil, fmt.Errorf("create container: %w", err)
	}
	containerID = strings.TrimSpace(containerID)

	return &Agent{
		ID:              containerID[:12],
		Name:            opts.Name,
		AgentID:         agentID,
		Status:          "running",
		TTYDPort:        port,
		CreatedAt:       time.Now(),
		Image:           image,
		Variant:         opts.Variant,
		ProjectID:       projectID,
		WorkspaceID:     workspaceID,
		ProjectHome:     paths.ProjectHome,
		ClaudeHome:      paths.ClaudeHome,
		CodexHome:       paths.CodexHome,
		AgentsHome:      paths.AgentsHome,
		AgentsHub:       paths.AgentsHub,
		ClaudeArgs:      opts.ClaudeArgs,
		HasAuthOverride: hasAuthOverride,
		HasBaseOverride: hasBaseOverride,
	}, nil
}

// RecreateAgent removes the existing container and creates a new one. Fields
// left empty in opts are inherited from the previous container's labels so
// callers can submit partial updates.
func (m *Manager) RecreateAgent(ctx context.Context, id string, opts CreateAgentOpts) (*Agent, error) {
	prev, err := m.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}

	if opts.Name == "" {
		opts.Name = prev.Name
	}
	if opts.AgentID == "" {
		opts.AgentID = prev.AgentID
	}
	if opts.Variant == "" {
		opts.Variant = prev.Variant
	}
	if opts.ProjectID == "" {
		opts.ProjectID = prev.ProjectID
	}
	if opts.WorkspaceID == "" {
		opts.WorkspaceID = prev.WorkspaceID
	}
	if opts.ProjectHome == "" {
		opts.ProjectHome = prev.ProjectHome
	}
	if opts.ClaudeHome == "" {
		opts.ClaudeHome = prev.ClaudeHome
	}
	if opts.CodexHome == "" {
		opts.CodexHome = prev.CodexHome
	}
	if opts.AgentsHome == "" {
		opts.AgentsHome = prev.AgentsHome
	}
	if opts.AgentsHub == "" {
		opts.AgentsHub = prev.AgentsHub
	}
	if opts.ClaudeArgs == nil {
		opts.ClaudeArgs = prev.ClaudeArgs
	}

	if err := m.RemoveAgent(ctx, prev.ID); err != nil {
		return nil, fmt.Errorf("remove previous container: %w", err)
	}

	return m.CreateAgent(ctx, opts)
}

func (m *Manager) RestartAgent(ctx context.Context, id string) error {
	agent, err := m.GetAgent(ctx, id)
	if err != nil {
		return err
	}
	if err := m.dockerRun(ctx, "restart", agent.ID); err != nil {
		return fmt.Errorf("restart container: %w", err)
	}
	return nil
}

func (m *Manager) ListAgents(ctx context.Context) ([]*Agent, error) {
	output, err := m.dockerOutput(ctx,
		"ps", "-a",
		"--filter", "label="+LabelManaged+"=true",
		"--format", `{"id":"{{.ID}}","name":"{{.Names}}","status":"{{.Status}}","image":"{{.Image}}","labels":"{{.Labels}}","created":"{{.CreatedAt}}"}`,
	)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	if output == "" {
		return []*Agent{}, nil
	}

	var agents []*Agent
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var raw struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Status  string `json:"status"`
			Image   string `json:"image"`
			Labels  string `json:"labels"`
			Created string `json:"created"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		labels := parseLabelString(raw.Labels)
		port, _ := parsePortFromLabel(labels)

		status := normalizeStatus(raw.Status)

		var claudeArgs []string
		if v := labels[LabelClaudeArgs]; v != "" {
			_ = json.Unmarshal([]byte(v), &claudeArgs)
		}

		agents = append(agents, &Agent{
			ID:              raw.ID,
			Name:            labels[LabelName],
			AgentID:         labels[LabelAgentID],
			Status:          status,
			TTYDPort:        port,
			CreatedAt:       parseDockerCreatedAt(raw.Created),
			Image:           raw.Image,
			Variant:         labels[LabelVariant],
			ProjectID:       labels[LabelProjectID],
			WorkspaceID:     labels[LabelWorkspaceID],
			ProjectHome:     labels[LabelProjectHome],
			ClaudeHome:      labels[LabelClaudeHome],
			CodexHome:       labels[LabelCodexHome],
			AgentsHome:      labels[LabelAgentsHome],
			AgentsHub:       labels[LabelAgentsHub],
			ClaudeArgs:      claudeArgs,
			HasAuthOverride: labels[LabelHasAuthOvr] == "true",
			HasBaseOverride: labels[LabelHasBaseOvr] == "true",
		})
	}
	return agents, nil
}

func (m *Manager) GetAgent(ctx context.Context, id string) (*Agent, error) {
	if id == "" {
		return nil, fmt.Errorf("empty agent id")
	}
	agents, err := m.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	var match *Agent
	for _, a := range agents {
		if a.ID == id {
			return a, nil
		}
		if strings.HasPrefix(a.ID, id) {
			if match != nil {
				return nil, fmt.Errorf("agent id %q is ambiguous", id)
			}
			match = a
		}
	}
	if match != nil {
		return match, nil
	}
	return nil, fmt.Errorf("agent %s not found", id)
}

func (m *Manager) RemoveAgent(ctx context.Context, id string) error {
	agent, err := m.GetAgent(ctx, id)
	if err != nil {
		return err
	}

	_ = m.dockerRun(ctx, "stop", "-t", "10", agent.ID)

	if err := m.dockerRun(ctx, "rm", "-f", agent.ID); err != nil {
		if !strings.Contains(err.Error(), "No such container") {
			return fmt.Errorf("remove container: %w", err)
		}
	}

	if agent.TTYDPort > 0 {
		m.ports.Release(agent.TTYDPort)
	}

	if agent.Name != "" {
		volume := "node_home_" + sanitizeName(agent.Name)
		_ = m.dockerRun(ctx, "volume", "rm", "-f", volume)
	}

	return nil
}

func (m *Manager) Close() error { return nil }

func (m *Manager) dockerRun(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

func (m *Manager) dockerOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func parseLabelString(s string) map[string]string {
	labels := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if k, v, ok := strings.Cut(pair, "="); ok {
			labels[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return labels
}

func parsePortFromLabel(labels map[string]string) (int, error) {
	s, ok := labels[LabelTTYDPort]
	if !ok {
		return 0, fmt.Errorf("no port label")
	}
	var p int
	fmt.Sscanf(s, "%d", &p)
	return p, nil
}

func sanitizeName(name string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name))
}

func normalizeStatus(s string) string {
	l := strings.ToLower(s)
	switch {
	case strings.HasPrefix(l, "up"):
		return "running"
	case strings.HasPrefix(l, "exited"):
		return "exited"
	case strings.HasPrefix(l, "created"):
		return "created"
	case strings.HasPrefix(l, "restarting"):
		return "restarting"
	}
	return "unknown"
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func joinShellQuoted(tokens []string) string {
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		parts = append(parts, shellSingleQuote(t))
	}
	return strings.Join(parts, " ")
}

// buildStartupScript renders the in-container bootstrap that starts a shpool
// session running claude, then exposes it via ttyd on the chosen port.
func buildStartupScript(wrappedClaudeCmd, workspace string, port int) string {
	return fmt.Sprintf(
		`export CLAUDE_CMD=%s; export AGENT_WORKSPACE=%s; export TERM="${TERM:-xterm-256color}"; export COLORTERM="${COLORTERM:-truecolor}"; export FORCE_COLOR="${FORCE_COLOR:-3}"; mkdir -p "$XDG_RUNTIME_DIR" && chmod 700 "$XDG_RUNTIME_DIR" && (umask 077; export -p > /tmp/agent-env.sh) && shpool attach -b -f --dir "$AGENT_WORKSPACE" -c "$CLAUDE_CMD" claude && for i in $(seq 1 50); do shpool list >/dev/null 2>&1 && break; sleep 0.1; done && printf '%%s\n' '#!/bin/bash' 'exec shpool attach -f claude' > /tmp/agent-cc-web-shpool && chmod +x /tmp/agent-cc-web-shpool && exec /usr/local/bin/ttyd -p %d -W /tmp/agent-cc-web-shpool`,
		shellSingleQuote(wrappedClaudeCmd),
		shellSingleQuote(workspace),
		port,
	)
}
