package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// WIPMonitor polls the kernel WIP accumulators once per second, converts the
// per-window byte/open totals into rates (MB/s, files/s), and feeds them to the
// observation-only ControllerV3. It shares the V2 BPFMonitor's loaded program
// (the WIP maps and the sys_enter_write tracepoint live in the same object).
type WIPMonitor struct {
	bm     *BPFMonitor
	ctrl   *ControllerV3
	stopCh chan struct{}
}

// NewWIPMonitor attaches the V3 write tracepoint and starts the poll loop.
func NewWIPMonitor(bm *BPFMonitor, ctrl *ControllerV3) (*WIPMonitor, error) {
	if err := bm.manager.AttachWIPTracepoint(); err != nil {
		return nil, err
	}
	w := &WIPMonitor{
		bm:     bm,
		ctrl:   ctrl,
		stopCh: make(chan struct{}),
	}
	go w.loop()
	return w, nil
}

func (w *WIPMonitor) loop() {
	const interval = time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	last := time.Now()
	for {
		select {
		case <-w.stopCh:
			return
		case now := <-ticker.C:
			elapsed := now.Sub(last).Seconds()
			last = now
			if elapsed <= 0 {
				elapsed = interval.Seconds()
			}

			samples, err := w.bm.manager.ReadWIP()
			if err != nil {
				log.Printf("⚠️  [V3] WIP read failed: %v", err)
				continue
			}

			for _, s := range samples {
				tbwMBs := (float64(s.TBWAccum) / 1e6) / elapsed
				ufmPerSec := float64(s.UFMAccum) / elapsed
				if tbwMBs == 0 && ufmPerSec == 0 {
					continue
				}
				w.ctrl.HandleWIPSample(int(s.PID), procComm(int(s.PID)), tbwMBs, ufmPerSec)
			}

			w.ctrl.Cleanup()
		}
	}
}

// Close stops the poll loop.
func (w *WIPMonitor) Close() {
	close(w.stopCh)
}

// procComm reads the process name from /proc/<pid>/comm. Returns "unknown" if
// the process has already exited.
func procComm(pid int) string {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}
