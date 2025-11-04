package enforcement

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// Throttler manages CPU throttling via cgroups v2
type Throttler struct {
	config  *Config
	checker *SafetyChecker

	// Track throttled processes
	throttled        map[int]time.Time
	throttleAttempts int // NEW: count attempts, not just unique PIDs
}

// NewThrottler creates a new throttler
func NewThrottler(config *Config, checker *SafetyChecker) *Throttler {
	return &Throttler{
		config:    config,
		checker:   checker,
		throttled: make(map[int]time.Time),
	}
}

// Throttle applies CPU limits to a process
func (t *Throttler) Throttle(pid int, cmd string, dwell time.Duration) error {
	// Check if already throttled recently
	if lastThrottle, exists := t.throttled[pid]; exists {
		if time.Since(lastThrottle) < 10*time.Second {
			return nil // Don't re-throttle too quickly
		}
	}

	// Safety check
	canEnforce, reason := t.checker.CanEnforce(pid, cmd)
	if !canEnforce {
		return fmt.Errorf("cannot throttle: %s", reason)
	}

	// Check threshold
	if dwell < t.config.ThrottleThreshold {
		return nil
	}

	fmt.Printf("🐌 Throttling PID=%d (%s) dwell=%.2fs -> %d%% CPU\n",
		pid, cmd, dwell.Seconds(), t.config.ThrottleCPUQuota)

	// Try cgroups v2 first
	if err := t.throttleCgroupV2(pid); err == nil {
		t.throttled[pid] = time.Now()
		t.throttleAttempts++ // NEW
		return nil
	}

	// Fallback to nice/renice
	if err := t.throttleNice(pid); err != nil {
		return err
	}

	t.throttled[pid] = time.Now()
	t.throttleAttempts++ // NEW
	return nil
}

// throttleCgroupV2 uses cgroups v2 for precise CPU control
func (t *Throttler) throttleCgroupV2(pid int) error {
	// Check if cgroups v2 is available
	cgroupPath := "/sys/fs/cgroup"
	if _, err := os.Stat(filepath.Join(cgroupPath, "cgroup.controllers")); err != nil {
		return fmt.Errorf("cgroups v2 not available: %w", err)
	}

	// Create dwell-fiber cgroup
	dwellCgroup := filepath.Join(cgroupPath, "dwell-fiber.slice")
	if err := os.MkdirAll(dwellCgroup, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup: %w", err)
	}

	// Enable CPU controller
	controllersPath := filepath.Join(cgroupPath, "cgroup.subtree_control")
	if err := os.WriteFile(controllersPath, []byte("+cpu"), 0644); err != nil {
		// May fail if already enabled, ignore
	}

	// Move process to cgroup
	procsPath := filepath.Join(dwellCgroup, "cgroup.procs")
	if err := os.WriteFile(procsPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("failed to move process to cgroup: %w", err)
	}

	// Set CPU quota (e.g., 20% = 20000 out of 100000)
	cpuMax := fmt.Sprintf("%d 100000", t.config.ThrottleCPUQuota*1000)
	cpuMaxPath := filepath.Join(dwellCgroup, "cpu.max")
	if err := os.WriteFile(cpuMaxPath, []byte(cpuMax), 0644); err != nil {
		return fmt.Errorf("failed to set CPU quota: %w", err)
	}

	return nil
}

// throttleNice uses nice/renice as fallback
func (t *Throttler) throttleNice(pid int) error {
	// Set nice value to +19 (lowest priority)
	if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, 19); err != nil {
		return fmt.Errorf("renice failed: %w", err)
	}
	return nil
}

// CleanupThrottled removes stale entries
func (t *Throttler) CleanupThrottled() {
	for pid, throttleTime := range t.throttled {
		// Remove if throttled > 1 minute ago and process is dead
		if time.Since(throttleTime) > time.Minute && !t.checker.IsAlive(pid) {
			delete(t.throttled, pid)
		}
	}
}

// GetThrottledCount returns number of currently throttled processes
func (t *Throttler) GetThrottledCount() int {
	return len(t.throttled)
}

// GetThrottleAttempts returns total throttle attempts (NEW)
func (t *Throttler) GetThrottleAttempts() int {
	return t.throttleAttempts
}
