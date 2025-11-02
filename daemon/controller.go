package main

import (
	"log"
	"math"
	"sync"
	"time"

	"github.com/dyb5784/dwell-fiber/pkg/bpf"
)

type Controller struct {
	mu         sync.RWMutex
	Alpha      float64
	Budget     float64
	Price      float64
	Dwell      float64
	UpdatedAt  time.Time
	Iteration  int
	Scenario   string
	UseRealBPF bool

	// BPF integration
	bpfManager *bpf.BPFManager
	dwellMap   map[uint32]float64 // PID -> accumulated dwell time (seconds)
}

func NewController(alpha, budget float64) *Controller {
	return &Controller{
		Alpha:      alpha,
		Budget:     budget,
		Price:      0.0,
		Dwell:      0.0,
		UpdatedAt:  time.Now(),
		Scenario:   "initializing",
		UseRealBPF: true, // Try real BPF first
		dwellMap:   make(map[uint32]float64),
	}
}

func (c *Controller) LoadBPF() error {
	log.Println("Attempting to load BPF program...")

	// Try to load real BPF
	bm, err := bpf.LoadBPF("bpf/dwell_monitor.bpf.o")
	if err != nil {
		log.Printf("⚠️  Failed to load BPF: %v", err)
		log.Println("Falling back to simulation mode")
		c.UseRealBPF = false
		c.Scenario = "simulation"
		return nil // Not a fatal error, fall back to simulation
	}

	// Attach to tracepoints
	if err := bm.AttachTracepoints(); err != nil {
		log.Printf("⚠️  Failed to attach tracepoints: %v", err)
		bm.Close()
		c.UseRealBPF = false
		c.Scenario = "simulation"
		return nil
	}

	// Start reader
	if err := bm.StartReader(); err != nil {
		log.Printf("⚠️  Failed to start reader: %v", err)
		bm.Close()
		c.UseRealBPF = false
		c.Scenario = "simulation"
		return nil
	}

	c.bpfManager = bm
	c.Scenario = "real-bpf"

	// Start processing events
	go c.processEvents(bm.Events)

	log.Println("✓ BPF integration active")
	return nil
}

func (c *Controller) processEvents(events <-chan bpf.DwellEvent) {
	for event := range events {
		c.mu.Lock()

		// Convert nanoseconds to seconds
		dwellSec := float64(event.DurationNs) / 1e9

		// Accumulate dwell time per PID
		c.dwellMap[event.PID] += dwellSec

		// Log interesting events (> 1 second)
		if dwellSec > 1.0 {
			comm := bpf.GetString(event.Comm[:])
			log.Printf("High dwell: PID=%d (%s) dwell=%.2fs", event.PID, comm, dwellSec)
		}

		c.mu.Unlock()
	}
}

func (c *Controller) Update() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.UseRealBPF {
		// Use real data from BPF
		c.updateFromBPF()
	} else {
		// Use simulation
		c.updateSimulation()
	}

	// ADMM price update: price = max(0, price + α*(dwell - budget))
	violation := c.Dwell - c.Budget
	newPrice := c.Price + c.Alpha*violation

	if newPrice < 0 {
		newPrice = 0
	}

	c.Price = newPrice
	c.UpdatedAt = time.Now()
	c.Iteration++
}

func (c *Controller) updateFromBPF() {
	// Calculate system-wide average dwell time
	if len(c.dwellMap) == 0 {
		c.Dwell = 0
		return
	}

	totalDwell := 0.0
	for _, dwell := range c.dwellMap {
		totalDwell += dwell
	}

	c.Dwell = totalDwell / float64(len(c.dwellMap))

	// Reset dwell map periodically (every 10 iterations)
	if c.Iteration%10 == 0 {
		c.dwellMap = make(map[uint32]float64)
	}
}

func (c *Controller) updateSimulation() {
	// Original simulation code
	t := float64(c.Iteration)
	cycle := (c.Iteration / 100) % 4

	switch cycle {
	case 0:
		c.Dwell = c.Budget + 2.0*math.Sin(t/10.0)
		c.Scenario = "simulation:normal"
	case 1:
		c.Dwell = c.Budget + 3.0 + 0.5*math.Sin(t/5.0)
		c.Scenario = "simulation:attack"
	case 2:
		progress := float64(c.Iteration%100) / 100.0
		c.Dwell = c.Budget + 3.0*(1.0-progress)
		c.Scenario = "simulation:recovery"
	case 3:
		c.Dwell = 1.5 + 0.5*math.Sin(t/15.0)
		c.Scenario = "simulation:idle"
	}

	if c.Dwell < 0 {
		c.Dwell = 0
	}
}

func (c *Controller) GetState() (float64, float64, time.Time, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Price, c.Dwell, c.UpdatedAt, c.Scenario
}

func (c *Controller) Close() error {
	if c.bpfManager != nil {
		return c.bpfManager.Close()
	}
	return nil
}
