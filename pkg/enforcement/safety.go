package enforcement

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// SafetyChecker validates if a process can be enforced
type SafetyChecker struct {
	config *Config
}

// NewSafetyChecker creates a new safety checker
func NewSafetyChecker(config *Config) *SafetyChecker {
	return &SafetyChecker{config: config}
}

// CanEnforce checks if enforcement is safe for this process
func (s *SafetyChecker) CanEnforce(pid int, cmd string) (bool, string) {
	// Check if enforcement is enabled
	if !s.config.Enabled {
		return false, "enforcement disabled (dry-run mode)"
	}

	// Check protected PIDs
	for _, protectedPID := range s.config.ProtectedPIDs {
		if pid == protectedPID {
			return false, fmt.Sprintf("PID %d is protected", pid)
		}
	}

	// Check if it's our own process
	if pid == os.Getpid() {
		return false, "cannot enforce on self"
	}

	// Check protected commands
	for _, protectedCmd := range s.config.ProtectedCmds {
		if strings.Contains(cmd, protectedCmd) {
			return false, fmt.Sprintf("command '%s' is protected", protectedCmd)
		}
	}

	// Check if process still exists
	if !isAlive(pid) {
		return false, "process no longer exists"
	}

	return true, ""
}

// IsAlive checks if a process is still running
func (s *SafetyChecker) IsAlive(pid int) bool {
	return isAlive(pid)
}

// isAlive uses signal 0 to probe for process existence.
// Returns true for nil (OK) or EPERM (exists but no permission), false for ESRCH (no such process).
func isAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM means it exists but we lack permission; treat as alive.
	if err == syscall.EPERM {
		return true
	}
	// ESRCH (or other) => not alive
	return false
}
