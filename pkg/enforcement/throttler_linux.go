//go:build linux

package enforcement

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// renicePID sets process nice value via Linux syscall
func (t *Throttler) renicePID(pid, niceValue int) error {
	// Use unix.Setpriority for Linux platforms
	// unix.PRIO_PROCESS = 0 (process priority)
	if err := unix.Setpriority(unix.PRIO_PROCESS, pid, niceValue); err != nil {
		return fmt.Errorf("renice failed: %w", err)
	}
	return nil
}
