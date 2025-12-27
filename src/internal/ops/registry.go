package ops

import (
	"fmt"
	"sync"
	"time"
)

// Registry holds all registered ops and provides lookup by ID.
type Registry struct {
	ops map[string]*Op
	mu  sync.RWMutex
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		ops: make(map[string]*Op),
	}
}

// Register adds an op to the registry.
// Panics if an op with the same ID is already registered.
func (r *Registry) Register(op *Op) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.ops[op.ID]; exists {
		panic(fmt.Sprintf("op %q already registered", op.ID))
	}
	r.ops[op.ID] = op
}

// Get returns an op by ID, or nil if not found.
func (r *Registry) Get(id string) *Op {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ops[id]
}

// MustGet returns an op by ID, panicking if not found.
func (r *Registry) MustGet(id string) *Op {
	op := r.Get(id)
	if op == nil {
		panic(fmt.Sprintf("op %q not registered", id))
	}
	return op
}

// All returns a copy of all registered ops.
func (r *Registry) All() []*Op {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Op, 0, len(r.ops))
	for _, op := range r.ops {
		result = append(result, op)
	}
	return result
}

// IDs returns all registered op IDs.
func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.ops))
	for id := range r.ops {
		result = append(result, id)
	}
	return result
}

// DefaultRegistry creates the standard NixFleet op registry.
// All ops from CORE-001 are registered here.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Host Ops (Agent-Executed)
	r.Register(opPull())
	r.Register(opSwitch())
	r.Register(opTest())
	r.Register(opRestart())
	r.Register(opStop())
	r.Register(opReboot())
	r.Register(opCheckVersion())
	r.Register(opRefreshGit())
	r.Register(opRefreshLock())
	r.Register(opRefreshSystem())
	r.Register(opBumpFlake())
	r.Register(opForceRebuild())

	// Dashboard Ops (Server-Side)
	r.Register(opMergePR())
	r.Register(opSetColor())
	r.Register(opRemove())

	return r
}

// ═══════════════════════════════════════════════════════════════════════════
// HOST OPS (Agent-Executed) — CORE-001 Table
// ═══════════════════════════════════════════════════════════════════════════

func opPull() *Op {
	return &Op{
		ID:          "pull",
		Description: "Git fetch + reset to latest",
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			if host.HasPendingCommand() {
				return &ValidationError{"busy", fmt.Sprintf("Command %q already running", host.GetPendingCommand())}
			}
			// Pull is meaningful if git is outdated (or unknown)
			if host.GetGitStatus() == "ok" {
				return &ValidationError{"already_current", "Git already up to date"}
			}
			return nil
		},
		PostCheck: func(host Host) *ValidationError {
			if host.GetLockStatus() != "ok" {
				return &ValidationError{"lock_not_ok", "Lock status not ok after pull"}
			}
			return nil
		},
		Timeout:        2 * time.Minute,
		WarningTimeout: 1 * time.Minute,
		Retryable:      true,
		Executor:       ExecutorAgent,
	}
}

func opSwitch() *Op {
	return &Op{
		ID:          "switch",
		Description: "nixos-rebuild/hm switch",
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			if host.HasPendingCommand() {
				return &ValidationError{"busy", fmt.Sprintf("Command %q already running", host.GetPendingCommand())}
			}
			// Check prerequisites: lock should be ok (or force)
			lock := host.GetLockStatus()
			if lock == "outdated" {
				return &ValidationError{"lock_outdated", "Pull required before switch (lock outdated)"}
			}
			// Check if switch is meaningful
			if host.GetSystemStatus() == "ok" && !host.IsAgentOutdated() {
				return &ValidationError{"already_current", "System already up to date"}
			}
			return nil
		},
		PostCheck: func(host Host) *ValidationError {
			if host.GetSystemStatus() != "ok" {
				return &ValidationError{"system_not_ok", "System status not ok after switch"}
			}
			// Agent freshness checked separately via reconnection
			return nil
		},
		Timeout:        10 * time.Minute,
		WarningTimeout: 5 * time.Minute,
		Retryable:      true,
		Executor:       ExecutorAgent,
	}
}

func opTest() *Op {
	return &Op{
		ID:          "test",
		Description: "Run host tests",
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			if host.HasPendingCommand() {
				return &ValidationError{"busy", fmt.Sprintf("Command %q already running", host.GetPendingCommand())}
			}
			return nil
		},
		Timeout:        5 * time.Minute,
		WarningTimeout: 2 * time.Minute,
		Retryable:      true,
		Executor:       ExecutorAgent,
	}
}

func opRestart() *Op {
	return &Op{
		ID:          "restart",
		Description: "Restart agent service",
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			return nil
		},
		PostCheck: func(host Host) *ValidationError {
			// Agent should reconnect
			if !host.IsOnline() {
				return &ValidationError{"not_reconnected", "Agent did not reconnect after restart"}
			}
			return nil
		},
		Timeout:        1 * time.Minute,
		WarningTimeout: 30 * time.Second,
		Retryable:      false, // Be careful with restart
		Executor:       ExecutorAgent,
	}
}

func opStop() *Op {
	return &Op{
		ID:          "stop",
		Description: "Stop running command",
		Validate: func(host Host) *ValidationError {
			if !host.HasPendingCommand() {
				return &ValidationError{"no_pending", "No command running to stop"}
			}
			return nil
		},
		PostCheck: func(host Host) *ValidationError {
			if host.HasPendingCommand() {
				return &ValidationError{"still_running", "Command still running after stop"}
			}
			return nil
		},
		Timeout:   30 * time.Second,
		Retryable: false,
		Executor:  ExecutorAgent,
	}
}

func opReboot() *Op {
	return &Op{
		ID:           "reboot",
		Description:  "System reboot",
		RequiresTotp: true,
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			// TOTP validation happens at handler level
			return nil
		},
		PostCheck: func(host Host) *ValidationError {
			// Agent should reconnect after reboot
			if !host.IsOnline() {
				return &ValidationError{"not_reconnected", "Agent did not reconnect after reboot"}
			}
			return nil
		},
		Timeout:        5 * time.Minute,
		WarningTimeout: 2 * time.Minute,
		Retryable:      false, // Reboot is disruptive
		Executor:       ExecutorAgent,
	}
}

func opCheckVersion() *Op {
	return &Op{
		ID:          "check-version",
		Description: "Compare running vs installed agent version",
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			return nil
		},
		Timeout:   10 * time.Second,
		Retryable: true,
		Executor:  ExecutorAgent,
	}
}

func opRefreshGit() *Op {
	return &Op{
		ID:          "refresh-git",
		Description: "Check GitHub for updates",
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			return nil
		},
		Timeout:   30 * time.Second,
		Retryable: true,
		Executor:  ExecutorAgent,
	}
}

func opRefreshLock() *Op {
	return &Op{
		ID:          "refresh-lock",
		Description: "Compare flake.lock",
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			return nil
		},
		Timeout:   30 * time.Second,
		Retryable: true,
		Executor:  ExecutorAgent,
	}
}

func opRefreshSystem() *Op {
	return &Op{
		ID:          "refresh-system",
		Description: "nix build --dry-run to check for changes",
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			return nil
		},
		Timeout:        5 * time.Minute,
		WarningTimeout: 2 * time.Minute,
		Retryable:      true,
		Executor:       ExecutorAgent,
	}
}

func opBumpFlake() *Op {
	return &Op{
		ID:          "bump-flake",
		Description: "nix flake update nixfleet",
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			return nil
		},
		PostCheck: func(host Host) *ValidationError {
			// Lock should change after bump
			if host.GetLockStatus() != "ok" {
				return &ValidationError{"lock_unchanged", "Lock file did not change after bump"}
			}
			return nil
		},
		Timeout:        2 * time.Minute,
		WarningTimeout: 1 * time.Minute,
		Retryable:      true,
		Executor:       ExecutorAgent,
	}
}

func opForceRebuild() *Op {
	return &Op{
		ID:          "force-rebuild",
		Description: "Rebuild with cache bypass",
		Validate: func(host Host) *ValidationError {
			if !host.IsOnline() {
				return &ValidationError{"offline", "Host is offline"}
			}
			return nil
		},
		PostCheck: func(host Host) *ValidationError {
			// Agent should be fresh after force rebuild
			if host.IsAgentOutdated() {
				return &ValidationError{"still_outdated", "Agent still outdated after force rebuild"}
			}
			return nil
		},
		Timeout:        15 * time.Minute,
		WarningTimeout: 10 * time.Minute,
		Retryable:      true,
		Executor:       ExecutorAgent,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// DASHBOARD OPS (Server-Side) — CORE-001 Table
// ═══════════════════════════════════════════════════════════════════════════

func opMergePR() *Op {
	return &Op{
		ID:          "merge-pr",
		Description: "Merge GitHub PR",
		Validate: func(host Host) *ValidationError {
			// PR mergeability checked at execution time
			return nil
		},
		PostCheck: func(host Host) *ValidationError {
			// PR should be merged
			return nil
		},
		Timeout:   30 * time.Second,
		Retryable: true,
		Executor:  ExecutorDashboard,
	}
}

func opSetColor() *Op {
	return &Op{
		ID:          "set-color",
		Description: "Update theme color",
		Validate: func(host Host) *ValidationError {
			// No preconditions
			return nil
		},
		Timeout:   5 * time.Second,
		Retryable: true,
		Executor:  ExecutorDashboard,
	}
}

func opRemove() *Op {
	return &Op{
		ID:          "remove",
		Description: "Remove host from database",
		Validate: func(host Host) *ValidationError {
			if host.IsOnline() {
				return &ValidationError{"online", "Cannot remove online host"}
			}
			return nil
		},
		Timeout:   5 * time.Second,
		Retryable: false, // Destructive
		Executor:  ExecutorDashboard,
	}
}

