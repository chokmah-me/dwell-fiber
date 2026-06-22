package enforcement

import (
	"fmt"
	"time"
)

// Enforcer coordinates throttling and killing
type Enforcer struct {
	config    *Config
	checker   *SafetyChecker
	throttler *Throttler
	killer    *Killer
}

// NewEnforcer creates a new enforcer
func NewEnforcer(config *Config) *Enforcer {
	checker := NewSafetyChecker(config)
	return &Enforcer{
		config:    config,
		checker:   checker,
		throttler: NewThrottler(config, checker),
		killer:    NewKiller(config, checker),
	}
}

// Enforce applies appropriate enforcement action
func (e *Enforcer) Enforce(pid int, cmd string, dwell time.Duration) error {
	// Try kill first (if threshold exceeded)
	if dwell >= e.config.KillThreshold {
		if err := e.killer.Kill(pid, cmd, dwell); err != nil {
			fmt.Printf("⚠️  Kill failed: %v\n", err)
		}
		return nil
	}

	// Try throttle (if threshold exceeded)
	if dwell >= e.config.ThrottleThreshold {
		if err := e.throttler.Throttle(pid, cmd, dwell); err != nil {
			fmt.Printf("⚠️  Throttle failed: %v\n", err)
		}
		return nil
	}

	return nil
}

// EnforceWIP applies enforcement based on the V3 rate-based ADMM price rather
// than a dwell duration. Mirrors Enforce: kill above V3KillPrice, else io.max
// throttle above V3ThrottlePrice. Dry-run (config.Enabled / KillEnabled) and
// safety gating are handled by the killer/throttler, so when those flags are off
// this only logs the action it *would* take.
func (e *Enforcer) EnforceWIP(pid int, cmd string, price float64) error {
	if price >= e.config.V3KillPrice {
		if err := e.killer.KillNow(pid, cmd, fmt.Sprintf("v3 price=%.0f", price)); err != nil {
			fmt.Printf("⚠️  V3 kill failed: %v\n", err)
		}
		return nil
	}

	if price >= e.config.V3ThrottlePrice {
		// Throttle only takes kernel action when enforcement is enabled; otherwise
		// log the intent so dry-run still shows what would happen.
		if !e.config.Enabled {
			fmt.Printf("🐌 [DRY-RUN] Would io-throttle PID=%d (%s) v3 price=%.0f\n", pid, cmd, price)
			return nil
		}
		if err := e.throttler.ThrottleIO(pid, cmd, fmt.Sprintf("v3 price=%.0f", price)); err != nil {
			fmt.Printf("⚠️  V3 throttle failed: %v\n", err)
		}
		return nil
	}

	return nil
}

// Cleanup removes stale tracking data
func (e *Enforcer) Cleanup() {
	e.throttler.CleanupThrottled()
	e.killer.CleanupKilled()
}

// GetStats returns enforcement statistics
func (e *Enforcer) GetStats() (throttled, killed int) {
	return e.throttler.GetThrottledCount(), e.killer.GetKilledCount()
}

// GetConfig returns the current configuration
func (e *Enforcer) GetConfig() *Config {
	return e.config
}

// SetEnabled enables/disables enforcement
func (e *Enforcer) SetEnabled(enabled bool) {
	e.config.Enabled = enabled
}

// SetKillEnabled enables/disables process killing
func (e *Enforcer) SetKillEnabled(killEnabled bool) {
	e.config.KillEnabled = killEnabled
}

// SetThrottleThreshold sets the throttle threshold
func (e *Enforcer) SetThrottleThreshold(threshold time.Duration) {
	e.config.ThrottleThreshold = threshold
}

// SetKillThreshold sets the kill threshold
func (e *Enforcer) SetKillThreshold(threshold time.Duration) {
	e.config.KillThreshold = threshold
}
