//go:build !linux

package enforcement

import "fmt"

// renicePID is a no-op placeholder on non-Linux platforms.
// The real implementation lives in throttler_linux.go.
func (t *Throttler) renicePID(pid, niceValue int) error {
	return fmt.Errorf("renice not implemented for this platform")
}
