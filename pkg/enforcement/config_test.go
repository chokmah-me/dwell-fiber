package enforcement

import "testing"

// TestCalibratedV3Thresholds pins the V3 price thresholds to the WSL
// calibration (pass-3 2026-09-25, confirmed 2026-09-28 with the paced bench:
// P_b=0.0, ambient idle 0.0, P_i=371.5/387.7; band M=0.15, R=2.0).
// Changing them requires re-running test/wsl_acp_validate.sh and updating
// this test deliberately -- never "fix" a failure by editing the numbers.
func TestCalibratedV3Thresholds(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.V3ThrottlePrice != 102.2 {
		t.Errorf("V3ThrottlePrice = %v, want 102.2 (calibrated 2026-09-28)",
			cfg.V3ThrottlePrice)
	}
	if cfg.V3KillPrice != 204.4 {
		t.Errorf("V3KillPrice = %v, want 204.4 (calibrated 2026-09-28)",
			cfg.V3KillPrice)
	}
	if cfg.V3KillPrice <= cfg.V3ThrottlePrice {
		t.Errorf("V3KillPrice (%v) must exceed V3ThrottlePrice (%v)",
			cfg.V3KillPrice, cfg.V3ThrottlePrice)
	}
}
