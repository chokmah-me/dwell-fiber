package enforcement

import "time"

// Config holds enforcement configuration
type Config struct {
	// Enable enforcement (false = dry-run mode)
	Enabled bool

	// Throttle settings
	ThrottleThreshold time.Duration // Dwell time to trigger throttle
	ThrottleCPUQuota  int           // CPU percentage (0-100)

	// Kill settings
	KillThreshold time.Duration // Dwell time to trigger kill
	KillEnabled   bool          // Actually kill processes

	// V3 (WIP) enforcement settings. V3 decisions key off the rate-based ADMM
	// price (a unitless float), not a dwell duration, so these live alongside
	// the V2 duration thresholds rather than replacing them.
	V3ThrottlePrice float64 // V3 ADMM price to trigger io.max throttle
	V3KillPrice     float64 // V3 ADMM price to trigger kill
	V3ThrottleWBPS  int     // io.max write-bytes-per-second cap when throttling

	// Safety settings
	ProtectedPIDs []int    // PIDs that can never be touched
	ProtectedCmds []string // Commands that can never be touched
}

// DefaultConfig returns safe default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:           false,            // Start in dry-run
		ThrottleThreshold: 3 * time.Second,  // Changed from 5s
		ThrottleCPUQuota:  15,               // Changed from 20%
		KillThreshold:     10 * time.Second, // Changed from 15s
		KillEnabled:       false,            // Very conservative default
		// Calibrated 2026-09-25 (pass-3, WSL BPF host), confirmed 2026-09-28.
		// Formula (test/calibrate_v3.py, M=0.15, R=2.0):
		//   throttle = P_b + M*(P_i - P_b) = 0 + 0.15*681.37 = 102.2
		//   kill = throttle * R = 204.4
		// Confirmation runs (paced bench, 307-333 files/s): P_b=0.0,
		// ambient idle 0.0, P_i=371.5 and 387.7 -- both clear kill by ~1.8x;
		// benign/ambient stay under throttle (worst ambient seen: ~96 on
		// 09-25, source unidentified -- the storm detector labels it if it
		// returns). Do NOT change these without re-running
		// test/wsl_acp_validate.sh: TestCalibratedV3Thresholds pins them.
		V3ThrottlePrice: 102.2,   // throttle once sustained WIP pushes price up
		V3KillPrice:     204.4,   // kill only well past the throttle band
		V3ThrottleWBPS:  1048576, // 1 MB/s write cap (not 0 -- avoid hard hangs)
		ProtectedPIDs:     []int{1},         // init/systemd
		ProtectedCmds: []string{
			"systemd", "init", "sshd", "dbus-daemon",
			"NetworkManager", "gdm", "Xorg", "wayland",
		},
	}
}
