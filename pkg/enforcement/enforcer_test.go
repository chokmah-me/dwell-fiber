package enforcement

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

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

// NoSuchPID is guaranteed absent so no kernel action can occur; the
// "[io] Throttling" intent line prints before any cgroup work, so stdout
// still records whether the throttle branch ran.
const NoSuchPID = 1 << 30

// TestEnforceWIPThrottleNotShadowedByKillBand is the item-6 regression test:
// with killing disarmed, a price that jumps straight past BOTH bands in one
// window must still be throttled. Before the fix the kill band was checked
// first and returned early, so the throttle never fired and the attack ran
// unimpeded while the log filled with dry-run kill lines.
func TestEnforceWIPThrottleNotShadowedByKillBand(t *testing.T) {
	e := NewEnforcer(throttleOnlyConfig())
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
	e := NewEnforcer(throttleOnlyConfig())
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
