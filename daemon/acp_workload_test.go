package main

import (
	"math"
	"testing"
)

// TestEstimator_PhaseWorkloadProgression drives the REAL ACPPhaseEstimator
// with the per-window (tbw, ufm, wip) sequences the WSL phase workload
// (test/wsl_phase_workload.py) is designed to produce, and asserts the
// estimator walks recon -> learning -> exploitation. This is the executable
// spec for the live run: if it passes here, the WSL session should observe
// dwell_fiber_v3_acp_phase = 1 -> 2 -> 3.
//
// T2 numbers (python3 is not in tierByName -> T2): budget=150,
// wip = 0.3*tbw + 0.7*ufm.
func TestEstimator_PhaseWorkloadProgression(t *testing.T) {
	const budget = 150.0
	e := NewACPPhaseEstimator()
	const pid = 9001

	wipOf := func(tbw, ufm float64) float64 { return 0.3*tbw + 0.7*ufm }

	// Phase 1: paced enumeration, 150 opens/s, no writes. WIP=105 < budget.
	for i := 0; i < 15; i++ {
		if ph := e.Observe(pid, 0, 150, wipOf(0, 150), budget); ph != PhaseRecon {
			t.Fatalf("phase1 window %d: expected recon, got %s", i, ph)
		}
	}

	// Phase 2: over-budget file churn (create/write 8 KiB/delete) at ~280
	// files/s with shrinking per-window jitter: 55% -> 35% relative.
	// Deterministic alternating amplitudes stand in for the random jitter.
	// CV stays above the 0.25 exploitation line throughout this phase.
	var sawLearning int
	for i := 0; i < 24; i++ {
		amp := 154.0 - 2.5*float64(i) // 154 -> 96.5 (55% -> 34% of 280)
		sign := 1.0
		if i%2 == 1 {
			sign = -1
		}
		ufm := 280 + sign*amp
		tbw := 280 * 8.0 / 1024 // 8 KiB files -> ~2.2 MB/s
		ph := e.Observe(pid, tbw, ufm, wipOf(tbw, ufm), budget)
		if ph == PhaseLearning {
			sawLearning++
		}
		if ph == PhaseExploitation {
			t.Fatalf("phase2 window %d: hit exploitation too early (CV dipped under 0.25?)", i)
		}
	}
	if sawLearning < 3 {
		t.Fatalf("phase2: expected several learning windows, saw %d", sawLearning)
	}

	// Phase 3: steady ~280 files/s, jitter ~5% -> exploitation.
	var exploitStreak int
	for i := 0; i < 14; i++ {
		ufm := 280 + 5*math.Sin(float64(i)) // tiny deterministic wobble
		tbw := 280 * 8.0 / 1024
		ph := e.Observe(pid, tbw, ufm, wipOf(tbw, ufm), budget)
		if ph == PhaseExploitation {
			exploitStreak++
		} else {
			exploitStreak = 0
		}
	}
	if exploitStreak < 6 {
		t.Fatalf("phase3: expected sustained exploitation at the end, final streak %d", exploitStreak)
	}
}
