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

// Cleanup removes stale tracking data
func (e *Enforcer) Cleanup() {
	e.throttler.CleanupThrottled()
	e.killer.CleanupKilled()
}

// GetStats returns enforcement statistics
func (e *Enforcer) GetStats() (throttled, killed int) {
	return e.throttler.GetThrottledCount(), e.killer.GetKilledCount()
}
