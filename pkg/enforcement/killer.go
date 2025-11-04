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
	// Check if kill is enabled
	if !k.config.KillEnabled {
		fmt.Printf("💀 [DRY-RUN] Would kill PID=%d (%s) dwell=%.2fs\n",
			pid, cmd, dwell.Seconds())
		return nil
	}
	
	// Check if already killed recently
	if lastKill, exists := k.killed[pid]; exists {
		if time.Since(lastKill) < 30*time.Second {
			return nil
		}
	}
	
	// Safety check
	canEnforce, reason := k.checker.CanEnforce(pid, cmd)
	if !canEnforce {
		return fmt.Errorf("cannot kill: %s", reason)
	}
	
	// Check threshold
	if dwell < k.config.KillThreshold {
		return nil
	}
	
	fmt.Printf("💀 Killing PID=%d (%s) dwell=%.2fs (critical threshold)\n",
		pid, cmd, dwell.Seconds())
	
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
