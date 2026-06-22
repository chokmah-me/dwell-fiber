package enforcement

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// Killer manages process termination
type Killer struct {
	config  *Config
	checker *SafetyChecker
	
	// Track killed processes
	killed map[int]time.Time
}

// NewKiller creates a new killer
func NewKiller(config *Config, checker *SafetyChecker) *Killer {
	return &Killer{
		config: config,
		checker: checker,
		killed: make(map[int]time.Time),
	}
}

// Kill terminates a process gracefully then forcefully
func (k *Killer) Kill(pid int, cmd string, dwell time.Duration) error {
	// Check threshold (dry-run check happens in KillNow)
	if k.config.KillEnabled && dwell < k.config.KillThreshold {
		return nil
	}
	return k.KillNow(pid, cmd, fmt.Sprintf("dwell=%.2fs", dwell.Seconds()))
}

// KillNow terminates a process gracefully then forcefully, gated only by the
// KillEnabled dry-run flag and safety checks -- the caller owns the decision of
// whether the process warrants killing. `reason` is a human-readable cause
// string (e.g. "dwell=12.0s" or "v3 price=2100") used in log output. Shared by
// the V2 dwell path (Kill) and the V3 WIP path (Enforcer.EnforceWIP).
func (k *Killer) KillNow(pid int, cmd, reason string) error {
	// Check if kill is enabled
	if !k.config.KillEnabled {
		fmt.Printf("💀 [DRY-RUN] Would kill PID=%d (%s) %s\n", pid, cmd, reason)
		return nil
	}

	// Check if already killed recently
	if lastKill, exists := k.killed[pid]; exists {
		if time.Since(lastKill) < 30*time.Second {
			return nil
		}
	}

	// Safety check
	canEnforce, sreason := k.checker.CanEnforce(pid, cmd)
	if !canEnforce {
		return fmt.Errorf("cannot kill: %s", sreason)
	}

	fmt.Printf("💀 Killing PID=%d (%s) %s (critical threshold)\n", pid, cmd, reason)

	// Get process handle
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process not found: %w", err)
	}
	
	// Try SIGTERM first (graceful)
	fmt.Printf("   → Sending SIGTERM to PID=%d\n", pid)
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}
	
	// Wait 5 seconds for graceful shutdown
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !k.checker.IsAlive(pid) {
			fmt.Printf("   ✓ Process %d terminated gracefully\n", pid)
			k.killed[pid] = time.Now()
			return nil
		}
	}
	
	// Still alive, send SIGKILL (forceful)
	fmt.Printf("   → Sending SIGKILL to PID=%d (forced)\n", pid)
	if err := process.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to send SIGKILL: %w", err)
	}
	
	k.killed[pid] = time.Now()
	fmt.Printf("   ✓ Process %d terminated forcefully\n", pid)
	return nil
}

// CleanupKilled removes stale entries
func (k *Killer) CleanupKilled() {
	for pid, killTime := range k.killed {
		if time.Since(killTime) > 5*time.Minute {
			delete(k.killed, pid)
		}
	}
}

// GetKilledCount returns number of killed processes
func (k *Killer) GetKilledCount() int {
	return len(k.killed)
}
