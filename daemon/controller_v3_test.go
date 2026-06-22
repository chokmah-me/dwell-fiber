package main

import (
	"sync"
	"testing"

	"github.com/chokmah-me/dwell-fiber/pkg/enforcement"
)

// newTestControllerV3 builds a ControllerV3 without Prometheus registration by
// constructing the struct directly (NewControllerV3 calls MustRegister, which
// panics on a duplicate registration across tests). Mirrors the pattern in
// controller_test.go. The aggregate gauges are left nil; tests that touch state
// avoid publishPeak by calling the pure helpers (CalculateWIP, ClassifyTier,
// updatePriceV3) rather than HandleWIPSample.
func newTestControllerV3(alpha float64) *ControllerV3 {
	return &ControllerV3{
		Alpha:         alpha,
		mu:            sync.RWMutex{},
		processStates: make(map[int]*ProcessStateV3),
	}
}

func TestClassifyTier_NameBased(t *testing.T) {
	c := newTestControllerV3(0.5)
	cases := []struct {
		cmd  string
		want Tier
	}{
		{"rsync", T1},
		{"tar", T1},
		{"gcc", T1_5},
		{"npm", T1_5},
		{"docker build", T1_5}, // first token matches
		{"python3", T2},        // unknown -> strict default
		{"some-ransom", T2},
		{"", T2},
	}
	for _, tc := range cases {
		if got := c.ClassifyTier(tc.cmd); got != tc.want {
			t.Errorf("ClassifyTier(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

func TestCalculateWIP_PerTier(t *testing.T) {
	c := newTestControllerV3(0.5)

	// T2 weights: 0.3*TBW + 0.7*UFM. 500 MB/s, 500 files/s -> 500.
	if got := c.CalculateWIP(T2, 500, 500); got != 500 {
		t.Errorf("WIP(T2,500,500) = %.1f, want 500", got)
	}
	// T1 weights: 0.9*TBW + 0.1*UFM. 600 MB/s, 50 files/s -> 545.
	if got := c.CalculateWIP(T1, 600, 50); got != 545 {
		t.Errorf("WIP(T1,600,50) = %.1f, want 545", got)
	}
}

func TestUpdatePriceV3_RisesUnderAttack(t *testing.T) {
	c := newTestControllerV3(0.5)
	// Intermittent attack: T2 WIP 500 vs budget 300 -> violation +200.
	// price 0 + 0.5*200 = 100.
	price := c.updatePriceV3(0, 500, tierConfigs[T2].Budget)
	if price <= 0 {
		t.Fatalf("expected price to rise above 0 under attack, got %.3f", price)
	}
	if price != 100 {
		t.Errorf("expected price 100, got %.3f", price)
	}
	// Sustained pressure keeps climbing.
	if next := c.updatePriceV3(price, 500, tierConfigs[T2].Budget); next <= price {
		t.Errorf("expected price to keep climbing, got %.3f after %.3f", next, price)
	}
}

func TestUpdatePriceV3_StaysZeroWhenIdle(t *testing.T) {
	c := newTestControllerV3(0.5)
	// WIP 0 vs budget 300 -> large negative violation, clamped to 0.
	if price := c.updatePriceV3(0, 0, tierConfigs[T2].Budget); price != 0 {
		t.Errorf("expected price 0 when idle, got %.3f", price)
	}
	// Even with a little residual price, it decays to 0 (non-negative clamp).
	if price := c.updatePriceV3(50, 0, tierConfigs[T2].Budget); price != 0 {
		t.Errorf("expected price clamped to 0, got %.3f", price)
	}
}

func TestLeak_BleedsOffTransientBurst(t *testing.T) {
	c := newTestControllerV3(0.5)
	c.Leak = defaultLeak
	// A one-off benign burst latched some price; with no further pressure it
	// must bleed back to 0 so it never reaches the enforcement threshold.
	price := 100.0
	for i := 0; i < 60; i++ {
		price = c.leak(price)
	}
	if price != 0 {
		t.Errorf("transient price did not bleed off: got %.3f, want 0", price)
	}
}

func TestLeak_SustainedStaysHigh(t *testing.T) {
	c := newTestControllerV3(0.5)
	c.Leak = defaultLeak
	budget := tierConfigs[T2].Budget

	sustained := 0.0
	for i := 0; i < 20; i++ {
		sustained = c.updatePriceV3(c.leak(sustained), budget+1000, budget)
	}
	if sustained <= 0 {
		t.Fatalf("sustained pressure should keep price elevated, got %.3f", sustained)
	}

	// The same magnitude price, once idle, must fall below the sustained level.
	idle := sustained
	for i := 0; i < 20; i++ {
		idle = c.leak(idle)
	}
	if idle >= sustained {
		t.Errorf("idle price %.3f should fall below sustained %.3f", idle, sustained)
	}
}

func TestEnforceWIP_DryRunTakesNoAction(t *testing.T) {
	cfg := enforcement.DefaultConfig() // Enabled=false, KillEnabled=false
	e := enforcement.NewEnforcer(cfg)

	for _, price := range []float64{0, cfg.V3ThrottlePrice, cfg.V3KillPrice, cfg.V3KillPrice * 10} {
		if err := e.EnforceWIP(999999, "unknown-bin", price); err != nil {
			t.Errorf("EnforceWIP(price=%.0f) in dry-run errored: %v", price, err)
		}
	}
	if throttled, killed := e.GetStats(); throttled != 0 || killed != 0 {
		t.Errorf("dry-run acted: throttled=%d killed=%d, want 0/0", throttled, killed)
	}
}

func TestHandleWIPSample_RecordsState(t *testing.T) {
	c := NewControllerV3(0.5) // full constructor: exercises metric publishing
	c.HandleWIPSample(4242, "python3", 500, 500)

	wip, price, tier, ok := c.GetState(4242)
	if !ok {
		t.Fatal("expected state recorded for pid 4242")
	}
	if tier != T2 {
		t.Errorf("expected T2 for python3, got %v", tier)
	}
	if wip != 500 {
		t.Errorf("expected WIP 500, got %.1f", wip)
	}
	if price <= 0 {
		t.Errorf("expected price > 0 under attack-rate sample, got %.3f", price)
	}
}
