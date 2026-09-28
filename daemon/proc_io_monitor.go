package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ProcIOMonitor is a fallback WIP sensor that polls /proc/<pid>/io when BPF
// tracepoints don't fire (e.g., WSL kernels where sys_enter_write doesn't
// trigger for user processes). It computes per-process write rates and feeds
// them to the V3 controller as WIP samples.
type ProcIOMonitor struct {
	ctrl     *ControllerV3
	stopCh   chan struct{}
	prevIO   map[int]uint64 // pid -> previous write_bytes
	prevTime time.Time
}

// NewProcIOMonitor starts the /proc poll loop.
func NewProcIOMonitor(ctrl *ControllerV3) *ProcIOMonitor {
	m := &ProcIOMonitor{
		ctrl:     ctrl,
		stopCh:   make(chan struct{}),
		prevIO:   make(map[int]uint64),
		prevTime: time.Now(),
	}
	go m.loop()
	return m
}

func (m *ProcIOMonitor) Stop() {
	close(m.stopCh)
}

func (m *ProcIOMonitor) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.poll()
		}
	}
}

// readWriteBytes reads /proc/<pid>/io and returns the write_bytes counter.
func readWriteBytes(pid int) (uint64, bool) {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "io"))
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "write_bytes:") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				v, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return v, true
				}
			}
		}
	}
	return 0, false
}

// readComm reads /proc/<pid>/comm.
func readComm(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

func (m *ProcIOMonitor) poll() {
	now := time.Now()
	elapsed := now.Sub(m.prevTime).Seconds()
	if elapsed <= 0 {
		return
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}

	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue // not a PID
		}
		writeBytes, ok := readWriteBytes(pid)
		if !ok {
			continue
		}
		prev, exists := m.prevIO[pid]
		m.prevIO[pid] = writeBytes
		if !exists {
			continue // need two samples to compute rate
		}
		delta := writeBytes - prev
		if delta == 0 {
			continue // no I/O, skip
		}
		// Create a WIP sample with the delta as TBW.
		// TBW is bytes per window; our window is ~1 second.
		// Convert to MB/s for the controller.
		tbwMBps := float64(delta) / (1024 * 1024) / elapsed
		comm := readComm(pid)
		m.ctrl.HandleWIPSample(pid, comm, tbwMBps, 0)
	}

	// Clean up dead PIDs to avoid unbounded growth.
	for pid := range m.prevIO {
		if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); os.IsNotExist(err) {
			delete(m.prevIO, pid)
		}
	}

	m.prevTime = now
	fmt.Printf("[proc-io] Polled %d processes\n", len(m.prevIO))
}
