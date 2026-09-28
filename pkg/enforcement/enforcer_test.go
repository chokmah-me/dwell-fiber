package enforcement

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// mockChecker always allows enforcement (for testing band logic without
// needing live PIDs).
type mockChecker struct{}

func (m *mockChecker) CanEnforce(pid int, cmd string) (bool, string) {
	return true, ""
}

func (m *mockChecker) IsAlive(pid int) bool {
	return true
}

// captureOutput runs fn with os.Stdout redirected and returns what was printed.
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// throttleOnlyConfig returns an armed-throttle / disarmed-kill config like the
// item-6 live test (--v3-enforce without --v3-enable-killing).
func throttleOnlyConfig() *Config {
	c := DefaultConfig()
	c.Enabled = true
	c.KillEnabled = false
	return c
}

// NoSuchPID is guaranteed absent so no kernel action can occur. Note: the
// throttler now gracefully skips dead PIDs (returns nil without logging),
// so tests that verify the throttle *decision* logic must use a live PID.
const NoSuchPID = 1 << 30

// TestEnforceWIPThrottleNotShadowedByKillBand is the item-6 regression test:
// with killing disarmed, a price that jumps straight past BOTH bands in one
// window must still be throttled. Before the fix the kill band was checked
// first and returned early, so the throttle never fired and the attack ran
// unimpeded while the log filled with dry-run kill lines.
func TestEnforceWIPThrottleNotShadowedByKillBand(t *testing.T) {
	// Use a mock checker so the throttle intent is logged even for a fake PID.
	// This verifies the band decision logic, not the safety check.
	e := NewEnforcerWithChecker(throttleOnlyConfig(), &mockChecker{})
	out := captureOutput(func() {
		if err := e.EnforceWIP(NoSuchPID, "bench", 470.0); err != nil {
			t.Fatalf("EnforceWIP: %v", err)
		}
	})
	if !strings.Contains(out, "[io] Throttling") && !strings.Contains(out, "V3 throttle") {
		t.Errorf("throttle branch did not run for price above kill band (disarmed).\noutput:\n%s", out)
	}
	if !strings.Contains(out, "[DRY-RUN] Would kill") {
		t.Errorf("dry-run kill intent not logged for price above kill band.\noutput:\n%s", out)
	}
}

// TestEnforceWIPThrottleBandDisarmed: price between the bands throttles and
// does not log kill intent.
func TestEnforceWIPThrottleBandDisarmed(t *testing.T) {
	e := NewEnforcerWithChecker(throttleOnlyConfig(), &mockChecker{})
	out := captureOutput(func() {
		if err := e.EnforceWIP(NoSuchPID, "bench", 150.0); err != nil {
			t.Fatalf("EnforceWIP: %v", err)
		}
	})
	if !strings.Contains(out, "[io] Throttling") && !strings.Contains(out, "V3 throttle") {
		t.Errorf("throttle branch did not run for price in throttle band.\noutput:\n%s", out)
	}
	if strings.Contains(out, "Would kill") {
		t.Errorf("kill intent logged for price below kill band.\noutput:\n%s", out)
	}
}

// TestEnforceWIPKillPrecedenceArmed: with killing armed the kill band keeps
// precedence -- no throttle attempt for a price above the kill band.
func TestEnforceWIPKillPrecedenceArmed(t *testing.T) {
	c := throttleOnlyConfig()
	c.KillEnabled = true
	e := NewEnforcer(c)
	out := captureOutput(func() {
		if err := e.EnforceWIP(NoSuchPID, "bench", 470.0); err != nil {
			t.Fatalf("EnforceWIP: %v", err)
		}
	})
	if strings.Contains(out, "[io] Throttling") {
		t.Errorf("throttle branch ran in armed mode above kill band; kill must keep precedence.\noutput:\n%s", out)
	}
}

// TestEnforceWIPBelowThrottleBand: no enforcement at all below the throttle price.
func TestEnforceWIPBelowThrottleBand(t *testing.T) {
	e := NewEnforcer(throttleOnlyConfig())
	out := captureOutput(func() {
		if err := e.EnforceWIP(NoSuchPID, "bench", 50.0); err != nil {
			t.Fatalf("EnforceWIP: %v", err)
		}
	})
	if strings.Contains(out, "Throttling") || strings.Contains(out, "kill") {
		t.Errorf("enforcement fired below throttle band.\noutput:\n%s", out)
	}
}

// TestEnforceV2ThrottleNotShadowedByKillBand is the V2 analogue of the V3
// regression test: with killing disarmed, a dwell that exceeds BOTH thresholds
// must still be throttled. Before the fix the kill band was checked first and
// returned early; with killing disarmed the kill branch is log-only, so the
// process escaped containment entirely.
func TestEnforceV2ThrottleNotShadowedByKillBand(t *testing.T) {
	e := NewEnforcerWithChecker(throttleOnlyConfig(), &mockChecker{})
	out := captureOutput(func() {
		// 15s exceeds both ThrottleThreshold (3s) and KillThreshold (10s).
		if err := e.Enforce(NoSuchPID, "bench", 15*time.Second); err != nil {
			t.Fatalf("Enforce: %v", err)
		}
	})
	if !strings.Contains(out, "Throttling") {
		t.Errorf("throttle branch did not run for dwell above kill threshold (disarmed).\noutput:\n%s", out)
	}
	if !strings.Contains(out, "[DRY-RUN] Would kill") {
		t.Errorf("dry-run kill intent not logged for dwell above kill threshold.\noutput:\n%s", out)
	}
}
