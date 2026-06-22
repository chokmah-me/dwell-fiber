package main

import (
	"fmt"
	"log"
	"time"

	"github.com/chokmah-me/dwell-fiber/pkg/bpf"
)

// BPFMonitor wraps the BPF manager and connects events to the controller
type BPFMonitor struct {
	manager    *bpf.BPFManager
	controller *Controller
	stopCh     chan struct{}
}

// NewBPFMonitor creates a new BPF monitor and starts event processing
func NewBPFMonitor(controller *Controller) (*BPFMonitor, error) {
	manager, err := bpf.LoadBPF("bpf/dwell_monitor.bpf.o")
	if err != nil {
		return nil, fmt.Errorf("load BPF: %w", err)
	}

	if err := manager.AttachTracepoints(); err != nil {
		manager.Close()
		return nil, fmt.Errorf("attach tracepoints: %w", err)
	}

	if err := manager.StartReader(); err != nil {
		manager.Close()
		return nil, fmt.Errorf("start reader: %w", err)
	}

	bm := &BPFMonitor{
		manager:    manager,
		controller: controller,
		stopCh:     make(chan struct{}),
	}

	// Start event processor goroutine
	go bm.processEvents()

	return bm, nil
}

// processEvents reads from the BPF ring buffer and processes events
func (bm *BPFMonitor) processEvents() {
	eventCount := 0
	filteredCount := 0

	for {
		select {
		case <-bm.stopCh:
			return
		case event := <-bm.manager.Events:
			eventCount++
			durationSec := float64(event.DurationNs) / 1e9
			comm := bpf.GetString(event.Comm[:])

			// Filter out noise: only log events > 0.1 seconds
			if durationSec < 0.1 {
				filteredCount++
				if eventCount%100 == 0 {
					log.Printf("📡 Processed %d events (%d filtered < 0.1s)", eventCount, filteredCount)
				}
				continue
			}

			// Log significant events
			log.Printf("📥 Received event #%d (size: %d bytes)", eventCount, len(event.Comm))
			log.Printf("📊 Event details: PID=%d, Cmd=%s, Dwell=%.3fs", event.PID, comm, durationSec)

			// Track in controller - this will handle the dwell updates and filtering
			bm.controller.HandleCloseEvent(int(event.PID), comm, time.Duration(event.DurationNs))

			log.Printf("✓ Processed event for PID=%d (%s)", event.PID, comm)
		}
	}
}

// KernelStats returns the kernel-side pre-filter session counts: total counts
// every matched close (including sub-100ms dwells the ring buffer drops),
// filtered counts the subset dropped by the <100ms in-kernel filter.
func (bm *BPFMonitor) KernelStats() (total, filtered uint64, err error) {
	return bm.manager.ReadStats()
}

// Close stops the BPF monitor and cleans up resources
func (bm *BPFMonitor) Close() error {
	close(bm.stopCh)
	time.Sleep(100 * time.Millisecond) // Allow goroutine to exit
	return bm.manager.Close()
}
