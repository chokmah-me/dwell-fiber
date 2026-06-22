package enforcement

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
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

// ThrottleIO caps a process's write bandwidth via cgroups v2 io.max. Used by the
// V3 (WIP) path: WIP is a write-rate signal, so I/O throttling is the natural
// response (vs the CPU throttling of Throttle). Gated by safety checks and the
// recently-throttled debounce. `reason` is a log cause string. Falls back to CPU
// throttling if the io controller is unavailable.
func (t *Throttler) ThrottleIO(pid int, cmd, reason string) error {
	// Don't re-throttle too quickly.
	if lastThrottle, exists := t.throttled[pid]; exists {
		if time.Since(lastThrottle) < 10*time.Second {
			return nil
		}
	}

	canEnforce, sreason := t.checker.CanEnforce(pid, cmd)
	if !canEnforce {
		return fmt.Errorf("cannot throttle: %s", sreason)
	}

	fmt.Printf("🐌 [io] Throttling PID=%d (%s) %s -> %d B/s write\n",
		pid, cmd, reason, t.config.V3ThrottleWBPS)

	if err := t.throttleIOCgroupV2(pid); err != nil {
		// io.max unavailable (no io controller / can't resolve device): degrade
		// to CPU throttling rather than leaving the process unchecked.
		fmt.Printf("⚠️  io.max throttle failed (%v); falling back to CPU throttle\n", err)
		if cerr := t.throttleCgroupV2(pid); cerr != nil {
			if nerr := t.throttleNice(pid); nerr != nil {
				return nerr
			}
		}
	}

	t.throttled[pid] = time.Now()
	t.throttleAttempts++
	return nil
}

// throttleIOCgroupV2 caps write bandwidth for pid via cgroups v2 io.max in a
// dedicated dwell-fiber-v3.slice (separate from the CPU slice).
func (t *Throttler) throttleIOCgroupV2(pid int) error {
	cgroupPath := "/sys/fs/cgroup"
	if _, err := os.Stat(filepath.Join(cgroupPath, "cgroup.controllers")); err != nil {
		return fmt.Errorf("cgroups v2 not available: %w", err)
	}

	maj, min, err := backingDevice(pid)
	if err != nil {
		return err
	}

	v3Cgroup := filepath.Join(cgroupPath, "dwell-fiber-v3.slice")
	if err := os.MkdirAll(v3Cgroup, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup: %w", err)
	}

	// Enable the io controller on the subtree (ignore error if already enabled).
	_ = os.WriteFile(filepath.Join(cgroupPath, "cgroup.subtree_control"), []byte("+io"), 0644)

	procsPath := filepath.Join(v3Cgroup, "cgroup.procs")
	if err := os.WriteFile(procsPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("failed to move process to cgroup: %w", err)
	}

	ioMax := fmt.Sprintf("%d:%d wbps=%d", maj, min, t.config.V3ThrottleWBPS)
	if err := os.WriteFile(filepath.Join(v3Cgroup, "io.max"), []byte(ioMax), 0644); err != nil {
		return fmt.Errorf("failed to set io.max: %w", err)
	}
	return nil
}

// backingDevice returns the major:minor of the block device backing the process's
// current working directory (a proxy for where it is writing). Falls back to the
// root filesystem's device.
func backingDevice(pid int) (uint32, uint32, error) {
	var st unix.Stat_t
	if err := unix.Stat(fmt.Sprintf("/proc/%d/cwd", pid), &st); err != nil {
		if err := unix.Stat("/", &st); err != nil {
			return 0, 0, fmt.Errorf("stat backing device: %w", err)
		}
	}
	return unix.Major(st.Dev), unix.Minor(st.Dev), nil
}

// throttleNice uses nice/renice as fallback
func (t *Throttler) throttleNice(pid int) error {
	// Set nice value to +10 (reduce priority, lower CPU allocation)
	// Note: This uses platform-specific syscalls and only works on Linux
	return t.renicePID(pid, 10)
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
