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
		// STARTING POINTS, pending VM calibration (bench.py benign vs intermittent
		// is the gate -- see docs/v3-roadmap.md). The observed intermittent run
		// reached price ~145 without decay; with decay the attack's steady-state
		// price is lower, so these sit below that to still trigger. Re-tune so
		// benign/tar stays under V3ThrottlePrice and intermittent clears it.
		V3ThrottlePrice: 50,      // throttle once sustained WIP pushes price up
		V3KillPrice:     150,     // kill only well past the throttle band
		V3ThrottleWBPS:  1048576, // 1 MB/s write cap (not 0 -- avoid hard hangs)
		ProtectedPIDs:     []int{1},         // init/systemd
		ProtectedCmds: []string{
			"systemd", "init", "sshd", "dbus-daemon",
			"NetworkManager", "gdm", "Xorg", "wayland",
		},
	}
}
