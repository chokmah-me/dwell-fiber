package enforcement

import (
	"fmt"
	"os"
	"strings"
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
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, "process not found"
	}
	
	// Send signal 0 to check if process is alive
	err = process.Signal(os.Signal(nil))
	if err != nil {
		return false, "process no longer exists"
	}
	
	return true, ""
}

// IsAlive checks if a process is still running
func (s *SafetyChecker) IsAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	
	err = process.Signal(os.Signal(nil))
	return err == nil
}
