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
	
	// Safety settings
	ProtectedPIDs []int    // PIDs that can never be touched
	ProtectedCmds []string // Commands that can never be touched
}

// DefaultConfig returns safe default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:           false, // Start in dry-run
		ThrottleThreshold: 5 * time.Second,
		ThrottleCPUQuota:  20, // Limit to 20% CPU
		KillThreshold:     15 * time.Second,
		KillEnabled:       false, // Very conservative default
		ProtectedPIDs:     []int{1}, // init/systemd
		ProtectedCmds: []string{
			"systemd", "init", "sshd", "dbus-daemon",
			"NetworkManager", "gdm", "Xorg", "wayland",
		},
	}
}
