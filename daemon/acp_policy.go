package main

// ACP cognitive-phase price policy.
//
// This module is the dwell-fiber side of the ACP bridge. In the acp-simulation
// repo, the OptimisticACPDefender keys its response off attacker knowledge
// completeness: while the IBLT attacker's model is still learning (the
// cognitive latency window) it spends only cheap actions -- deception, honeypot,
// monitor -- and reserves expensive action for confirmed compromise. It never
// pays for RESTORE_NODE (cost 6.0) speculatively.
//
// The daemon cannot observe attacker knowledge directly, so this module ports
// the *decision logic* -- phase-contingent response -- as a Go-native policy:
// each PID's recent rate behavior is classified into an attacker phase, and the
// V3 ADMM price update is modulated per phase:
//
//	PhaseRecon        -> dampen (alpha x0.4, budget x1.25): do not punish
//	                   exploration; this is the latency-arbitrage window --
//	                   observe and let the attacker reveal its pattern.
//	PhaseLearning     -> near-normal (alpha x0.8, budget x1.0).
//	PhaseExploitation -> escalate (alpha x1.8, budget x0.85): a stable,
//	                   confident high-pressure pattern reaches the throttle /
//	                   kill thresholds faster.
//
// Calibration notes (from acp-simulation, 2026):
//   - OptimisticACPDefender default acp_strength = 0.65 (deception probability
//     during the latency window); CognitiveAttacker IBLT decay d = 0.8.
//   - The 8-window history ring below mirrors IBLT activation concentration:
//     as the attacker's memory model converges, observed behavior variance
//     shrinks -- the same signal the sim's defender uses to stop deceiving
//     and start responding.
//
// This is a heuristic port, not the full IBLT. Honest scope: the multipliers
// are starting points, unvalidated against live ransomware; see
// docs/acp-bridge.md. The policy is opt-in (--acp-policy) and V3-only; the V2
// dwell controller is untouched.

import "math"

// AttackerPhase is the daemon-side analogue of the ACP attacker's cognitive
// state.
type AttackerPhase int

const (
	PhaseUnknown AttackerPhase = iota
	PhaseRecon
	PhaseLearning
	PhaseExploitation
)

func (p AttackerPhase) String() string {
	switch p {
	case PhaseRecon:
		return "recon"
	case PhaseLearning:
		return "learning"
	case PhaseExploitation:
		return "exploitation"
	default:
		return "unknown"
	}
}

// Phase estimation tuning. All thresholds are documented starting points.
const (
	acpWindowN      = 8     // per-PID behavior history ring (windows)
	acpMinWindows   = 3     // windows before a phase past recon is possible
	reconOpensPerSec = 50.0 // enumeration signature: high opens/s ...
	reconTBWCap     = 1.0   // ... with ~zero bytes written (MB/s)
	exploitCV       = 0.25  // CV below this = stable, confident pattern
	exploitSustain  = 4     // consecutive over-budget windows for exploitation
)

// pidPhaseState is the per-PID observation history backing phase estimation.
type pidPhaseState struct {
	wips           [acpWindowN]float64
	n              int // windows observed (saturates at acpWindowN)
	idx            int // ring write index
	prevCV         float64
	hasPrevCV      bool
	overBudgetStreak int
}

// ACPPhaseEstimator infers the attacker phase per PID from recent rate
// behavior. It is deliberately stateless across PIDs and deterministic:
// the same window sequence always yields the same phase.
type ACPPhaseEstimator struct {
	states map[int]*pidPhaseState
}

// NewACPPhaseEstimator returns an empty estimator.
func NewACPPhaseEstimator() *ACPPhaseEstimator {
	return &ACPPhaseEstimator{states: make(map[int]*pidPhaseState)}
}

// Observe records one window for a PID and returns its estimated phase.
// tbw is MB/s, ufm is files/s, wip is the already-computed Weighted I/O
// Pressure, budget is the tier's WIP budget.
func (e *ACPPhaseEstimator) Observe(pid int, tbw, ufm, wip, budget float64) AttackerPhase {
	st, ok := e.states[pid]
	if !ok {
		st = &pidPhaseState{}
		e.states[pid] = st
	}
	st.wips[st.idx] = wip
	st.idx = (st.idx + 1) % acpWindowN
	if st.n < acpWindowN {
		st.n++
	}

	if wip > budget {
		st.overBudgetStreak++
	} else {
		st.overBudgetStreak = 0
	}

	reconSig := ufm >= reconOpensPerSec && tbw < reconTBWCap

	if st.n < acpMinWindows {
		// Cognitive latency window: too little history to judge convergence.
		// An enumeration signature still reads as recon; otherwise unknown.
		if reconSig {
			return PhaseRecon
		}
		return PhaseUnknown
	}

	mean, cv := wipStats(st)

	// Confident exploitation: stable high-pressure pattern sustained.
	if mean > budget && st.overBudgetStreak >= exploitSustain && cv <= exploitCV {
		st.prevCV, st.hasPrevCV = cv, true
		return PhaseExploitation
	}
	// Learning: behavior variance shrinking while pressure stays over budget --
	// the IBLT model converging on its target pattern.
	if mean > budget && st.hasPrevCV && cv < st.prevCV {
		st.prevCV = cv
		return PhaseLearning
	}
	st.prevCV, st.hasPrevCV = cv, true

	if reconSig {
		return PhaseRecon
	}
	return PhaseUnknown
}

// wipStats returns the mean and coefficient of variation of the stored WIP
// ring. A zero mean yields +Inf CV (no signal), never a false "stable".
func wipStats(st *pidPhaseState) (mean, cv float64) {
	n := st.n
	var sum float64
	for i := 0; i < n; i++ {
		sum += st.wips[i]
	}
	mean = sum / float64(n)
	if mean == 0 {
		return 0, math.Inf(1)
	}
	var sq float64
	for i := 0; i < n; i++ {
		d := st.wips[i] - mean
		sq += d * d
	}
	return mean, math.Sqrt(sq / float64(n)) / mean
}

// ACPPolicy maps an attacker phase to ADMM price-update multipliers.
// alphaMult scales the step size; budgetMult scales the tier budget.
type ACPPolicy struct {
	ReconAlphaMult, ReconBudgetMult       float64
	LearningAlphaMult, LearningBudgetMult float64
	ExploitAlphaMult, ExploitBudgetMult   float64
}

// DefaultACPPolicy returns the starting-point multipliers documented above.
func DefaultACPPolicy() *ACPPolicy {
	return &ACPPolicy{
		ReconAlphaMult:    0.4,
		ReconBudgetMult:   1.25,
		LearningAlphaMult: 0.8,
		LearningBudgetMult: 1.0,
		ExploitAlphaMult:  1.8,
		ExploitBudgetMult: 0.85,
	}
}

// Modulate returns the (alpha, budget) multipliers for a phase.
// PhaseUnknown is the identity: fixed pricing, today's behavior.
func (p *ACPPolicy) Modulate(phase AttackerPhase) (alphaMult, budgetMult float64) {
	switch phase {
	case PhaseRecon:
		return p.ReconAlphaMult, p.ReconBudgetMult
	case PhaseLearning:
		return p.LearningAlphaMult, p.LearningBudgetMult
	case PhaseExploitation:
		return p.ExploitAlphaMult, p.ExploitBudgetMult
	default:
		return 1.0, 1.0
	}
}
