package main

import (
	"sync"
	"testing"
)

// newACPPolicyTestController builds a ControllerV3 without Prometheus
// registration (struct literal -- NewControllerV3 would panic on duplicate
// MustRegister across tests). publishPeak is nil-safe, so HandleWIPSample can
// be driven end-to-end here.
func newACPPolicyTestController(alpha float64, withPolicy bool) *ControllerV3 {
	c := &ControllerV3{
		Alpha:         alpha,
		Leak:          defaultLeak,
		mu:            sync.RWMutex{},
		processStates: make(map[int]*ProcessStateV3),
	}
	if withPolicy {
		c.acpPolicy = DefaultACPPolicy()
		c.acpEstimator = NewACPPhaseEstimator()
	}
	return c
}

func TestModulate_Table(t *testing.T) {
	p := DefaultACPPolicy()
	cases := []struct {
		phase      AttackerPhase
		alphaMult  float64
		budgetMult float64
	}{
		{PhaseRecon, 0.4, 1.25},
		{PhaseLearning, 0.8, 1.0},
		{PhaseExploitation, 1.8, 0.85},
		{PhaseUnknown, 1.0, 1.0},
	}
	for _, tc := range cases {
		a, b := p.Modulate(tc.phase)
		if a != tc.alphaMult || b != tc.budgetMult {
			t.Errorf("Modulate(%v) = (%.2f, %.2f), want (%.2f, %.2f)",
				tc.phase, a, b, tc.alphaMult, tc.budgetMult)
		}
	}
}

func TestEstimator_ReconSignature(t *testing.T) {
	e := NewACPPhaseEstimator()
	// Enumeration signature: high opens/s, ~zero bytes written.
	if got := e.Observe(11, 0, 600, 420, 150); got != PhaseRecon {
		t.Errorf("first window with enumeration signature = %v, want recon", got)
	}
	if got := e.Observe(11, 0, 600, 420, 150); got != PhaseRecon {
		t.Errorf("second window with enumeration signature = %v, want recon", got)
	}
}

func TestEstimator_UnknownWithoutSignature(t *testing.T) {
	e := NewACPPhaseEstimator()
	// New PID, no enumeration signature, too little history: unknown.
	if got := e.Observe(12, 100, 100, 100, 150); got != PhaseUnknown {
		t.Errorf("got %v, want unknown", got)
	}
}

func TestEstimator_ConvergesToExploitation(t *testing.T) {
	e := NewACPPhaseEstimator()
	// Rapidly converging high-pressure pattern (probe-verified trace).
	wips := []float64{300, 500, 400, 450, 440, 445, 442, 441}
	want := []AttackerPhase{
		PhaseUnknown, PhaseUnknown, PhaseUnknown, PhaseExploitation,
		PhaseExploitation, PhaseExploitation, PhaseExploitation, PhaseExploitation,
	}
	for i, w := range wips {
		if got := e.Observe(13, 100, 100, w, 150); got != want[i] {
			t.Errorf("window %d: got %v, want %v", i+1, got, want[i])
		}
	}
}

func TestEstimator_LearningBeforeStable(t *testing.T) {
	e := NewACPPhaseEstimator()
	// Oscillating-then-settling pattern: variance shrinks before the pattern
	// is stable enough to count as exploitation (probe-verified trace).
	wips := []float64{900, 100, 700, 300, 600, 400, 550, 450}
	want := []AttackerPhase{
		PhaseUnknown, PhaseUnknown, PhaseUnknown, PhaseUnknown,
		PhaseLearning, PhaseLearning, PhaseLearning, PhaseLearning,
	}
	for i, w := range wips {
		if got := e.Observe(14, 100, 100, w, 150); got != want[i] {
			t.Errorf("window %d: got %v, want %v", i+1, got, want[i])
		}
	}
}

func TestEstimator_IdleStaysUnknown(t *testing.T) {
	e := NewACPPhaseEstimator()
	// Zero pressure: mean 0 -> CV +Inf, never learning/exploitation.
	for i := 0; i < 6; i++ {
		if got := e.Observe(15, 0, 0, 0, 150); got != PhaseUnknown {
			t.Fatalf("idle window %d: got %v, want unknown", i+1, got)
		}
	}
}

func TestPolicy_DampensDuringRecon(t *testing.T) {
	plain := newACPPolicyTestController(0.5, false)
	withACP := newACPPolicyTestController(0.5, true)

	// Enumeration-phase workload: tbw=0, ufm=600 -> T2 WIP 420, over budget.
	// First 3 windows stay in recon (insufficient history for convergence).
	for i := 0; i < 3; i++ {
		plain.HandleWIPSample(21, "evil-scan", 0, 600)
		withACP.HandleWIPSample(21, "evil-scan", 0, 600)
	}

	_, pPlain, _, _ := plain.GetState(21)
	_, pACP, _, _ := withACP.GetState(21)
	if !(pACP < pPlain) {
		t.Errorf("recon: policy price %.3f should be below fixed price %.3f (dampened alpha, raised budget)", pACP, pPlain)
	}
	if phase, _ := withACP.GetPhase(21); phase != PhaseRecon {
		t.Errorf("recon: phase = %v, want recon", phase)
	}
}

func TestPolicy_EscalatesDuringExploitation(t *testing.T) {
	plain := newACPPolicyTestController(0.5, false)
	withACP := newACPPolicyTestController(0.5, true)

	// Stable high-pressure workload: tbw=400, ufm=400 -> T2 WIP 400.
	for i := 0; i < 8; i++ {
		plain.HandleWIPSample(22, "evil-crypt", 400, 400)
		withACP.HandleWIPSample(22, "evil-crypt", 400, 400)
	}

	_, pPlain, _, _ := plain.GetState(22)
	_, pACP, _, _ := withACP.GetState(22)
	if !(pACP > pPlain) {
		t.Errorf("exploitation: policy price %.3f should exceed fixed price %.3f (escalated alpha, lowered budget)", pACP, pPlain)
	}
	if phase, _ := withACP.GetPhase(22); phase != PhaseExploitation {
		t.Errorf("exploitation: phase = %v, want exploitation", phase)
	}
}

func TestPolicy_DisabledIsIdentity(t *testing.T) {
	// Without the policy the controller behaves exactly as before: phase stays
	// unknown and the price matches the fixed-alpha update.
	c := newACPPolicyTestController(0.5, false)
	for i := 0; i < 5; i++ {
		c.HandleWIPSample(23, "evil-scan", 0, 600)
	}
	if phase, ok := c.GetPhase(23); !ok || phase != PhaseUnknown {
		t.Errorf("disabled policy: phase = %v, want unknown", phase)
	}
	_, price, _, _ := c.GetState(23)
	// Hand-computed fixed update: step 0.5*(420-150)=135, leak 0.9.
	want := 0.0
	for i := 0; i < 5; i++ {
		want = want * 0.9
		if want < 0.5 {
			want = 0
		}
		want += 135
	}
	if price != want {
		t.Errorf("disabled policy: price %.3f, want fixed-update %.3f", price, want)
	}
}
