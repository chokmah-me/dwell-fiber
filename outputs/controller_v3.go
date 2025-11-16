package main

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/dyb5784/dwell-fiber/pkg/enforcement"
	"github.com/prometheus/client_golang/prometheus"
)

// V3.0: Tier classification based on TBW/UFM ratio
type Tier int

const (
	T1   Tier = iota // High TBW, low UFM (backups) - ω₁=0.9, ω₂=0.1, Budget=12000
	T1_5             // Mixed workload (dev builds) - ω₁=0.55, ω₂=0.45, Budget=8000
	T2               // High UFM, varied TBW (untrusted) - ω₁=0.3, ω₂=0.7, Budget=4000
)

type TierConfig struct {
	Omega1 float64 // Weight for TBW
	Omega2 float64 // Weight for UFM
	Budget float64 // WIP budget for this tier
}

var tierConfigs = map[Tier]TierConfig{
	T1:   {Omega1: 0.9, Omega2: 0.1, Budget: 12000},
	T1_5: {Omega1: 0.55, Omega2: 0.45, Budget: 8000},
	T2:   {Omega1: 0.3, Omega2: 0.7, Budget: 4000},
}

type ControllerV3 struct {
	Alpha float64 // ADMM step size
	mu    sync.RWMutex

	// Per-process state
	processStates map[int]*ProcessState

	// Enforcement
	enforcer *enforcement.Enforcer

	// Metrics
	wipGauge          prometheus.Gauge
	priceGauge        prometheus.Gauge
	tierGauge         prometheus.Gauge
	tierSwitchCounter prometheus.Counter
}

type ProcessState struct {
	PID          int
	Cmd          string
	CurrentTier  Tier
	CurrentPrice float64
	LastUpdate   time.Time

	// V3.0 metrics
	TBW float64 // Total Bytes Written
	UFM float64 // Unique Files Modified
	WIP float64 // Weighted I/O Pressure
}

func NewControllerV3(alpha float64) *ControllerV3 {
	enfConfig := enforcement.DefaultConfig()

	c := &ControllerV3{
		Alpha:         alpha,
		processStates: make(map[int]*ProcessState),
		enforcer:      enforcement.NewEnforcer(enfConfig),
		wipGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_wip_current",
			Help: "Current Weighted I/O Pressure (TBW + UFM weighted)",
		}),
		priceGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_price",
			Help: "Current ADMM price",
		}),
		tierGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_tier",
			Help: "Current tier classification (1=T1, 1.5=T1.5, 2=T2)",
		}),
		tierSwitchCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dwell_fiber_tier_switches_total",
			Help: "Total number of tier switches",
		}),
	}

	prometheus.MustRegister(c.wipGauge)
	prometheus.MustRegister(c.priceGauge)
	prometheus.MustRegister(c.tierGauge)
	prometheus.MustRegister(c.tierSwitchCounter)

	return c
}

// ClassifyTier implements Trust Classification Module (TCM)
func (c *ControllerV3) ClassifyTier(tbw, ufm float64) Tier {
	// Avoid division by zero
	if ufm < 1 {
		ufm = 1
	}

	// T1: High TBW, low UFM (heuristic)
	if tbw >= 10000*1024*1024 || ufm <= 1000 {
		return T1
	}

	// T1.5: Mixed (high UFM + high TBW)
	if ufm >= 20000 && tbw >= 500*1024*1024 {
		return T1_5
	}

	// T2: Default (untrusted)
	return T2
}

// CalculateWIP computes Weighted I/O Pressure for a given tier
func (c *ControllerV3) CalculateWIP(tier Tier, tbw, ufm float64) float64 {
	config := tierConfigs[tier]
	return (config.Omega1 * tbw) + (config.Omega2 * ufm)
}

// HandleWIPEvent processes a V3.0 WIP event from eBPF
func (c *ControllerV3) HandleWIPEvent(pid int, cmd string, tbw, ufm float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get or create process state
	state, exists := c.processStates[pid]
	if !exists {
		state = &ProcessState{
			PID:          pid,
			Cmd:          cmd,
			CurrentTier:  T2, // Start as untrusted
			CurrentPrice: 0.1,
			LastUpdate:   time.Now(),
		}
		c.processStates[pid] = state
	}

	// Update metrics
	state.TBW = tbw
	state.UFM = ufm

	// Classify tier (TCM)
	oldTier := state.CurrentTier
	newTier := c.ClassifyTier(tbw, ufm)

	if oldTier != newTier {
		fmt.Printf("🔄 Tier switch: PID=%d (%s) %v → %v\n", pid, cmd, oldTier, newTier)
		c.tierSwitchCounter.Inc()
	}
	state.CurrentTier = newTier

	// Calculate WIP
	wip := c.CalculateWIP(newTier, tbw, ufm)
	state.WIP = wip

	// Get tier budget
	tierConfig := tierConfigs[newTier]
	budget := tierConfig.Budget

	// ADMM price update: π(t+1) = max(0, π(t) + α·(WIP(t) - Budget))
	violation := wip - budget
	newPrice := state.CurrentPrice + c.Alpha*violation
	state.CurrentPrice = math.Max(0, newPrice)
	state.LastUpdate = time.Now()

	// Log significant events
	if violation > 0 {
		fmt.Printf("⚠️  High WIP: PID=%d (%s) Tier=%v WIP=%.0f Budget=%.0f Price=%.4f\n",
			pid, cmd, newTier, wip, budget, state.CurrentPrice)
	}

	// Update metrics
	c.wipGauge.Set(wip)
	c.priceGauge.Set(state.CurrentPrice)
	c.tierGauge.Set(float64(newTier))

	// Enforcement bridge: estimate dwell from violation (temporary)
	estimatedDwell := time.Duration(violation) * time.Millisecond
	if estimatedDwell > 0 {
		c.enforcer.Enforce(pid, cmd, estimatedDwell)
	}
}

// GetState returns current state for a process
func (c *ControllerV3) GetState(pid int) (wip, price float64, tier Tier, updated time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if state, exists := c.processStates[pid]; exists {
		return state.WIP, state.CurrentPrice, state.CurrentTier, state.LastUpdate
	}

	return 0, 0, T2, time.Now()
}

// Cleanup removes stale process states
func (c *ControllerV3) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute)
	for pid, state := range c.processStates {
		if state.LastUpdate.Before(cutoff) {
			delete(c.processStates, pid)
		}
	}
}
