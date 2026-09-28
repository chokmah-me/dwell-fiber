package main

import (
	"testing"
	"time"
)

func TestAmbientStorm_MetronomicFreshPIDs(t *testing.T) {
	d := NewAmbientDetector()
	base := time.Now()
	// 6 bursts, ~30 s apart, fresh PID each time: the observed ambient shape.
	for i := 0; i < 6; i++ {
		d.ObserveBurst(1000+i, base.Add(time.Duration(i)*30*time.Second))
	}
	if !d.StormActive() {
		t.Fatal("expected storm active for metronomic fresh-PID bursts")
	}
	if d.TotalBursts() != 6 {
		t.Fatalf("expected 6 total bursts, got %d", d.TotalBursts())
	}
}

func TestAmbientStorm_IrregularIntervals(t *testing.T) {
	d := NewAmbientDetector()
	base := time.Now()
	// Same fresh-PID churn, but attacker-irregular timing.
	gaps := []time.Duration{0, 12 * time.Second, 95 * time.Second, 140 * time.Second, 161 * time.Second, 400 * time.Second}
	for i, g := range gaps {
		d.ObserveBurst(2000+i, base.Add(g))
	}
	if d.StormActive() {
		t.Fatal("expected no storm for irregular burst intervals")
	}
}

func TestAmbientStorm_PersistentPID(t *testing.T) {
	d := NewAmbientDetector()
	base := time.Now()
	// Metronomic bursts, but ONE persistent PID: per-PID logic owns this case,
	// the detector must stay out of it.
	for i := 0; i < 6; i++ {
		d.ObserveBurst(4242, base.Add(time.Duration(i)*30*time.Second))
	}
	if d.StormActive() {
		t.Fatal("expected no storm: persistent PID fails the churn condition")
	}
}

func TestAmbientStorm_TooFewBursts(t *testing.T) {
	d := NewAmbientDetector()
	base := time.Now()
	for i := 0; i < 3; i++ {
		d.ObserveBurst(3000+i, base.Add(time.Duration(i)*30*time.Second))
	}
	if d.StormActive() {
		t.Fatal("expected no storm with fewer than 4 bursts")
	}
}

func TestAmbientStorm_Decay(t *testing.T) {
	d := NewAmbientDetector()
	base := time.Now()
	for i := 0; i < 6; i++ {
		d.ObserveBurst(4000+i, base.Add(time.Duration(i)*30*time.Second))
	}
	if !d.StormActive() {
		t.Fatal("precondition: storm should be active")
	}
	// Silence for > 3x the ~30 s period: the storm must clear.
	d.Poll(base.Add(6*30*time.Second + 3*31*time.Second))
	if d.StormActive() {
		t.Fatal("expected storm to decay after 3x-period silence")
	}
}

func TestAmbientStorm_JitterWithinTolerance(t *testing.T) {
	d := NewAmbientDetector()
	base := time.Now()
	// ~30 s period with small jitter: still metronomic (CV well under 0.25).
	gaps := []time.Duration{0, 29 * time.Second, 61 * time.Second, 90 * time.Second, 121 * time.Second, 149 * time.Second}
	for i, g := range gaps {
		d.ObserveBurst(5000+i, base.Add(g))
	}
	if !d.StormActive() {
		t.Fatal("expected storm: jittered-but-metronomic bursts should still qualify")
	}
}

func TestAmbientStorm_PeriodOutOfRange(t *testing.T) {
	d := NewAmbientDetector()
	base := time.Now()
	// Metronomic but far too slow (10-minute cadence): not infrastructure-like.
	for i := 0; i < 6; i++ {
		d.ObserveBurst(6000+i, base.Add(time.Duration(i)*10*time.Minute))
	}
	if d.StormActive() {
		t.Fatal("expected no storm: period outside the infrastructure window")
	}
}

// TestAmbientWiring_ObserveOnly feeds recon-signature samples through
// HandleWIPSample with the detector on and off: prices must be identical,
// proving the detector never touches the price update.
func TestAmbientWiring_ObserveOnly(t *testing.T) {
	build := func(withAmbient bool) *ControllerV3 {
		c := newACPPolicyTestController(0.5, false)
		if withAmbient {
			c.ambient = NewAmbientDetector()
		}
		return c
	}
	drive := func(c *ControllerV3) {
		for i := 0; i < 10; i++ {
			// Recon signature: high opens/s, ~zero writes.
			c.HandleWIPSample(7000+i, "unknown", 0.0, 300.0)
		}
	}
	on := build(true)
	off := build(false)
	drive(on)
	drive(off)
	if on.ambient.TotalBursts() != 10 {
		t.Fatalf("expected 10 observed bursts, got %d", on.ambient.TotalBursts())
	}
	for pid, stOn := range on.processStates {
		stOff := off.processStates[pid]
		if stOff == nil {
			t.Fatalf("pid %d missing from detector-off controller", pid)
		}
		if stOn.CurrentPrice != stOff.CurrentPrice {
			t.Fatalf("pid %d: price differs with detector on (%.4f) vs off (%.4f)",
				pid, stOn.CurrentPrice, stOff.CurrentPrice)
		}
	}
}
