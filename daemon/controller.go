package main

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chokmah-me/dwell-fiber/pkg/enforcement"
	"github.com/prometheus/client_golang/prometheus"
)

// eventStatsProvider returns the cumulative (total, filtered) session counts to
// publish on the events_total / events_filtered_total metrics. It is pluggable
// so the authoritative source can be the kernel (BPF mode, where the counts
// include sub-100ms sessions the userspace pipeline never sees) or the
// controller's own counters (simulation mode, no kernel).
type eventStatsProvider func() (total, filtered uint64)

type Controller struct {
	Alpha  float64 // Exported for metrics
	Budget float64 // Exported for metrics
	mu     sync.RWMutex

	// State
	currentPrice float64
	lastUpdate   time.Time
	scenario     string

	// Real BPF tracking
	dwellMap map[int]*DwellInfo // PID -> dwell info

	// Recent dwell times for averaging
	recentDwells []float64
	maxRecent    int

	// Enforcement
	enforcer *enforcement.Enforcer

	// Metrics
	dwellGauge      prometheus.Gauge
	priceGauge      prometheus.Gauge
	throttledGauge  prometheus.Gauge
	killedGauge     prometheus.Gauge
	enforcementMode prometheus.Gauge

	// Pre-filter session counters. localTotal counts every event the controller
	// is handed; subSecFiltered counts those dropped by the sub-1s noise filter.
	// In BPF mode these are superseded as the metric source by the kernel's
	// pre-filter counts (see statsProvider), but subSecFiltered is still added
	// to the kernel's <100ms count so events_filtered_total reflects ALL
	// sessions that never moved the price (kernel <100ms + controller <1s).
	localTotal     atomic.Uint64
	subSecFiltered atomic.Uint64

	// statsProvider feeds the events_total / events_filtered_total CounterFuncs.
	// Defaults to the local counters; main.go swaps in a kernel-backed provider
	// once BPF loads.
	statsProvider atomic.Pointer[eventStatsProvider]
}

type DwellInfo struct {
	PID       int
	Cmd       string
	OpenTime  time.Time
	CloseTime time.Time
	Dwell     time.Duration
}

func NewController(alpha, budget float64) *Controller {
	// Create enforcement config - default to DISABLED (observation mode)
	enfConfig := enforcement.DefaultConfig()
	// Note: Enabled flag is set by CLI flags in main.go
	// Default is safe: observation mode, no enforcement
	enfConfig.ThrottleThreshold = 5 * time.Second
	enfConfig.ThrottleCPUQuota = 20
	enfConfig.KillThreshold = 15 * time.Second
	enfConfig.KillEnabled = false

	c := &Controller{
		Alpha:        alpha,
		Budget:       budget,
		currentPrice: 0.1,
		lastUpdate:   time.Now(),
		scenario:     "real-bpf",
		dwellMap:     make(map[int]*DwellInfo),
		recentDwells: make([]float64, 0),
		maxRecent:    10, // Keep last 10 measurements
		enforcer:     enforcement.NewEnforcer(enfConfig),
		dwellGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_dwell_time",
			Help: "Current average file dwell time (seconds)",
		}),
		priceGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_price",
			Help: "Current access price",
		}),
		throttledGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_throttled_count",
			Help: "Number of currently throttled processes",
		}),
		killedGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_killed_count",
			Help: "Number of killed processes",
		}),
		enforcementMode: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_enforcement_enabled",
			Help: "Enforcement mode (0=dry-run, 1=enabled)",
		}),
	}

	// Default to the local counters; replaced by a kernel-backed provider in
	// BPF mode (SetEventStatsProvider).
	c.useLocalStatsProvider()

	prometheus.MustRegister(c.dwellGauge)
	prometheus.MustRegister(c.priceGauge)
	prometheus.MustRegister(c.throttledGauge)
	prometheus.MustRegister(c.killedGauge)
	prometheus.MustRegister(c.enforcementMode)

	// events_total / events_filtered_total are pull-based CounterFuncs so the
	// kernel's pre-filter session counts can back them in BPF mode. This is what
	// makes fast-intermittent encryption (all sub-100ms dwells) observable:
	// every session is counted in-kernel even though none reach userspace.
	prometheus.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "dwell_fiber_events_total",
		Help: "Total file-close sessions observed, counted before any noise filter (kernel pre-filter count in BPF mode)",
	}, func() float64 {
		total, _ := c.eventStats()
		return float64(total)
	}))
	prometheus.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
		Name: "dwell_fiber_events_filtered_total",
		Help: "Sessions dropped by a noise filter and so never updated the price (kernel <100ms + controller <1s); subset of events_total",
	}, func() float64 {
		_, filtered := c.eventStats()
		return float64(filtered)
	}))

	c.SyncEnforcementMode()

	return c
}

// eventStats returns the current (total, filtered) counts from the active
// provider. Safe if no provider is set (returns zero).
func (c *Controller) eventStats() (total, filtered uint64) {
	if p := c.statsProvider.Load(); p != nil {
		return (*p)()
	}
	return 0, 0
}

// useLocalStatsProvider points the events metrics at the controller's own
// counters (simulation / no-BPF mode).
func (c *Controller) useLocalStatsProvider() {
	p := eventStatsProvider(func() (uint64, uint64) {
		return c.localTotal.Load(), c.subSecFiltered.Load()
	})
	c.statsProvider.Store(&p)
}

// SetEventStatsProvider installs a custom source for the events metrics. main.go
// uses this in BPF mode to report the kernel's pre-filter session counts (with
// the controller's sub-1s drops folded into the filtered total).
func (c *Controller) SetEventStatsProvider(fn eventStatsProvider) {
	c.statsProvider.Store(&fn)
}

// SubSecondFiltered returns the count of events the controller dropped via its
// sub-1s noise filter. Used by the BPF stats provider to fold the controller's
// 100ms-1s drops into events_filtered_total alongside the kernel's <100ms drops.
func (c *Controller) SubSecondFiltered() uint64 {
	return c.subSecFiltered.Load()
}

// SyncEnforcementMode publishes the current enforcement config to the
// dwell_fiber_enforcement_enabled gauge. Call after enforcement flags are
// applied: the gauge is otherwise only updated inside HandleCloseEvent's
// post-noise-filter path, so a daemon that processes only sub-1s dwells would
// never report its true enforcement state (it would read 0 / "dry-run" even
// with --enable-enforcement).
func (c *Controller) SyncEnforcementMode() {
	if c.enforcer.GetConfig().Enabled {
		c.enforcementMode.Set(1.0)
	} else {
		c.enforcementMode.Set(0.0)
	}
}

// Replace the HandleCloseEvent and related functions:

func (c *Controller) HandleCloseEvent(pid int, cmd string, dwell time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Count every event the controller is handed, before its own filtering.
	// In simulation mode this is the events_total source; in BPF mode the
	// kernel's pre-filter count supersedes it (the kernel also sees the sub-100ms
	// sessions that never reach here). Either way subSecFiltered below feeds the
	// filtered total.
	c.localTotal.Add(1)

	// Filter out noise: only process events > 1 seconds (as suggested)
	if dwell < 1*time.Second {
		c.subSecFiltered.Add(1)
		return // Silently skip noise
	}

	// Log high-dwell events for visibility
	fmt.Printf("⏱️  High dwell: PID=%d (%s) dwell=%.2fs\n", pid, cmd, dwell.Seconds())

	// Store dwell time for averaging
	dwellSeconds := dwell.Seconds()
	c.recentDwells = append(c.recentDwells, dwellSeconds)

	// Keep only recent measurements - THIS IS KEY: limit to last maxRecent events
	// This makes the average responsive to recent changes
	if len(c.recentDwells) > c.maxRecent {
		c.recentDwells = c.recentDwells[1:]
	}

	// Apply enforcement for significant dwell
	if dwell > time.Second {
		if err := c.enforcer.Enforce(pid, cmd, dwell); err != nil {
			fmt.Printf("⚠️  Enforcement error: %v\n", err)
		}
	}

	// Update metrics
	avgDwell := c.calculateAverageDwell()
	c.dwellGauge.Set(avgDwell)

	// Update price using ADMM
	c.updatePrice(avgDwell)
	c.priceGauge.Set(c.currentPrice)
	c.lastUpdate = time.Now()

	// Update enforcement metrics
	throttled, killed := c.enforcer.GetStats()
	c.throttledGauge.Set(float64(throttled))
	c.killedGauge.Set(float64(killed))
	if c.enforcer.GetConfig().Enabled {
		c.enforcementMode.Set(1.0) // Enforcement enabled
	} else {
		c.enforcementMode.Set(0.0) // Observation mode (safe default)
	}
}

func (c *Controller) calculateAverageDwell() float64 {
	if len(c.recentDwells) == 0 {
		return 0
	}

	var total float64
	for _, dwell := range c.recentDwells {
		total += dwell
	}

	return total / float64(len(c.recentDwells))
}

func (c *Controller) updatePrice(avgDwell float64) {
	// ADMM price update: p(t+1) = p(t) + α(d(t) - budget)
	// This should DECREASE when dwell < budget
	violation := avgDwell - c.Budget
	newPrice := c.currentPrice + c.Alpha*violation
	c.currentPrice = math.Max(0, newPrice)
}

func (c *Controller) RunCleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()

		// Cleanup enforcement tracking
		c.enforcer.Cleanup()

		// Cleanup old dwell entries
		cutoff := time.Now().Add(-1 * time.Minute)
		for pid, info := range c.dwellMap {
			if info.OpenTime.Before(cutoff) && info.CloseTime.IsZero() {
				delete(c.dwellMap, pid)
			}
		}

		c.mu.Unlock()
	}
}

// GetState returns state for metrics (4 values for compatibility)
func (c *Controller) GetState() (price float64, dwell float64, updated time.Time, scenario string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentPrice, c.calculateAverageDwell(), c.lastUpdate, c.scenario
}
