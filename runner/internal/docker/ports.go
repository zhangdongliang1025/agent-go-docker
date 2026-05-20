package docker

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	LabelManaged     = "agent-go-runner.managed"
	LabelName        = "agent-go-runner.name"
	LabelAgentID     = "agent-go-runner.agent-id"
	LabelTTYDPort    = "agent-go-runner.ttyd-port"
	LabelVariant     = "agent-go-runner.variant"
	LabelProjectID   = "agent-go-runner.project-id"
	LabelWorkspaceID = "agent-go-runner.workspace-id"
	LabelImage       = "agent-go-runner.image"
	LabelClaudeArgs  = "agent-go-runner.claude-args"
	LabelHasAuthOvr  = "agent-go-runner.auth-override"
	LabelHasBaseOvr  = "agent-go-runner.base-override"
	LabelProjectHome = "agent-go-runner.project-home"
	LabelClaudeHome  = "agent-go-runner.claude-home"
	LabelCodexHome   = "agent-go-runner.codex-home"
	LabelAgentsHome  = "agent-go-runner.agents-home"
	LabelAgentsHub   = "agent-go-runner.agents-hub"
)

type PortAllocator struct {
	mu        sync.Mutex
	allocated map[int]bool
	start     int
	end       int
}

func NewPortAllocator(start, end int) *PortAllocator {
	return &PortAllocator{
		allocated: make(map[int]bool),
		start:     start,
		end:       end,
	}
}

func (pa *PortAllocator) Allocate() (int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	for p := pa.start; p <= pa.end; p++ {
		if pa.allocated[p] {
			continue
		}
		if !isHostPortFree(p) {
			continue
		}
		pa.allocated[p] = true
		return p, nil
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", pa.start, pa.end)
}

// isHostPortFree checks whether the runner can bind to the port on the host,
// avoiding collisions with other host processes when containers use --network=host.
func isHostPortFree(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return false
	}
	return true
}

func (pa *PortAllocator) Release(port int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	delete(pa.allocated, port)
}

func (pa *PortAllocator) Reserve(port int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	pa.allocated[port] = true
}

func (pa *PortAllocator) RecoverFromLabels(labels map[string]string) error {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	portStr, ok := labels[LabelTTYDPort]
	if !ok {
		return nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port label %q: %w", portStr, err)
	}
	pa.allocated[port] = true
	return nil
}
