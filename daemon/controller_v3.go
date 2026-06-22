package main

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ControllerV3 is the V3.0 rate-based (Weighted I/O Pressure) controller. It runs
// in OBSERVATION ONLY mode alongside the V2 dwell controller: it computes a WIP
// signal and an ADMM price per process and publishes them as metrics, but never
// enforces. Its purpose is to make fast intermittent encryption -- which V2's
// dwell-latency tracking filters out -- visible as a rising signal, flipping the
// `intermittent` benchmark row from price~0 to detection.
//
// Enforcement (cgroups io.max throttling / killing), true unique-inode UFM, and
// tier-weight/budget calibration are deferred to a later phase. The tier weights
// below follow docs/v3-roadmap.md; the BUDGETS are MVP placeholders chosen so the
// signal is meaningful against the current crude TBW (MB/s) + UFM (opens/s) proxy
// -- they are NOT calibrated for false positives (the benign/tar scenario will
// also elevate WIP). That calibration is the deferred enforcement phase.
type Tier int

const (
	T1   Tier = iota // High-throughput legitimate (rsync, tar, dd)
	T1_5             // Development workloads (gcc, npm, cargo, docker)
	T2               // Untrusted / unknown (default)
)

func (t Tier) String() string {
	switch t {
	case T1:
		return "T1"
	case T1_5:
		return "T1.5"
	default:
		return "T2"
	}
}

// tierValue maps a tier to the numeric value published on the tier gauge.
func (t Tier) value() float64 {
	switch t {
	case T1:
		return 1.0
	case T1_5:
		return 1.5
	default:
		return 2.0
	}
}

type TierConfig struct {
	Omega1 float64 // weight for TBW (MB/s)
	Omega2 float64 // weight for UFM (files/s)
	Budget float64 // WIP budget (MVP placeholder; see type doc)
}

var tierConfigs = map[Tier]TierConfig{
	T1:   {Omega1: 0.9, Omega2: 0.1, Budget: 3000},
	T1_5: {Omega1: 0.55, Omega2: 0.45, Budget: 1500},
	T2:   {Omega1: 0.3, Omega2: 0.7, Budget: 300},
}

// Name-based tier classification (docs/v3-roadmap.md). Unknown processes default
// to T2 (strict). Replaces the draft's ratio heuristic, which mislabeled any
// low-file-count process as trusted T1.
var tierByName = map[string]Tier{
	"rsync": T1, "tar": T1, "dd": T1, "cp": T1, "backup": T1,
	"gcc": T1_5, "cc": T1_5, "clang": T1_5, "npm": T1_5,
	"cargo": T1_5, "go": T1_5, "docker": T1_5, "make": T1_5,
}

type ProcessStateV3 struct {
	PID          int
	Cmd          string
	CurrentTier  Tier
	CurrentPrice float64
	TBW          float64 // MB/s, last window
	UFM          float64 // files/s, last window
	WIP          float64
	LastUpdate   time.Time
}

type ControllerV3 struct {
	Alpha float64 // ADMM step size
	mu    sync.RWMutex

	processStates map[int]*ProcessStateV3

	// Aggregate "worst offender" gauges so the dashboard/bench can scrape a
	// single number; per-process detail stays in processStates.
	wipGauge          prometheus.Gauge
	priceGauge        prometheus.Gauge
	tbwGauge          prometheus.Gauge
	ufmGauge          prometheus.Gauge
	tierGauge         prometheus.Gauge
	tierSwitchCounter prometheus.Counter
}

func NewControllerV3(alpha float64) *ControllerV3 {
	c := &ControllerV3{
		Alpha:         alpha,
		processStates: make(map[int]*ProcessStateV3),
		wipGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_v3_wip",
			Help: "V3 Weighted I/O Pressure of the highest-pressure process (observation only)",
		}),
		priceGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_v3_price",
			Help: "V3 ADMM price of the highest-priced process (observation only, no enforcement)",
		}),
		tbwGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_v3_tbw",
			Help: "V3 total bytes written rate (MB/s) of the highest-pressure process",
		}),
		ufmGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_v3_ufm",
			Help: "V3 unique-files-modified rate (opens/s proxy) of the highest-pressure process",
		}),
		tierGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dwell_fiber_v3_tier",
			Help: "V3 tier of the highest-pressure process (1=T1, 1.5=T1.5, 2=T2)",
		}),
		tierSwitchCounter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dwell_fiber_v3_tier_switches_total",
			Help: "Total V3 tier reclassifications",
		}),
	}

	prometheus.MustRegister(c.wipGauge)
	prometheus.MustRegister(c.priceGauge)
	prometheus.MustRegister(c.tbwGauge)
	prometheus.MustRegister(c.ufmGauge)
	prometheus.MustRegister(c.tierGauge)
	prometheus.MustRegister(c.tierSwitchCounter)

	return c
}

// ClassifyTier maps a process name to a tier (name-based; unknown -> T2).
func (c *ControllerV3) ClassifyTier(cmd string) Tier {
	name := strings.ToLower(strings.TrimSpace(cmd))
	if t, ok := tierByName[name]; ok {
		return t
	}
	// Match on a prefix token too (e.g. "docker build" -> "docker").
	if fields := strings.Fields(name); len(fields) > 0 {
		if t, ok := tierByName[fields[0]]; ok {
			return t
		}
	}
	return T2
}

// CalculateWIP computes Weighted I/O Pressure for a tier. tbw is MB/s, ufm is
// files/s.
func (c *ControllerV3) CalculateWIP(tier Tier, tbw, ufm float64) float64 {
	cfg := tierConfigs[tier]
	return cfg.Omega1*tbw + cfg.Omega2*ufm
}

// updatePriceV3 applies the ADMM update for a single process: price stays >= 0.
func (c *ControllerV3) updatePriceV3(price, wip, budget float64) float64 {
	return math.Max(0, price+c.Alpha*(wip-budget))
}

// HandleWIPSample processes one per-PID, per-second rate sample. tbw is MB/s,
// ufm is files/s. Observation only: updates state + metrics, never enforces.
func (c *ControllerV3) HandleWIPSample(pid int, cmd string, tbw, ufm float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	st, ok := c.processStates[pid]
	if !ok {
		st = &ProcessStateV3{
			PID:          pid,
			Cmd:          cmd,
			CurrentTier:  c.ClassifyTier(cmd),
			CurrentPrice: 0,
			LastUpdate:   time.Now(),
		}
		c.processStates[pid] = st
	}

	newTier := c.ClassifyTier(cmd)
	if newTier != st.CurrentTier {
		c.tierSwitchCounter.Inc()
		st.CurrentTier = newTier
	}

	st.TBW = tbw
	st.UFM = ufm
	st.WIP = c.CalculateWIP(st.CurrentTier, tbw, ufm)
	st.CurrentPrice = c.updatePriceV3(st.CurrentPrice, st.WIP, tierConfigs[st.CurrentTier].Budget)
	st.LastUpdate = time.Now()

	if st.CurrentPrice > 0 {
		fmt.Printf("📈 [V3] High I/O pressure: PID=%d (%s) tier=%s TBW=%.1fMB/s UFM=%.0f/s WIP=%.0f price=%.3f (observation only)\n",
			pid, cmd, st.CurrentTier, tbw, ufm, st.WIP, st.CurrentPrice)
	}

	c.publishPeak()
}

// publishPeak sets the aggregate gauges to the current highest-price process.
// Caller must hold c.mu.
func (c *ControllerV3) publishPeak() {
	var peak *ProcessStateV3
	for _, st := range c.processStates {
		if peak == nil || st.CurrentPrice > peak.CurrentPrice {
			peak = st
		}
	}
	if peak == nil {
		return
	}
	c.wipGauge.Set(peak.WIP)
	c.priceGauge.Set(peak.CurrentPrice)
	c.tbwGauge.Set(peak.TBW)
	c.ufmGauge.Set(peak.UFM)
	c.tierGauge.Set(peak.CurrentTier.value())
}

// Cleanup removes stale process states and refreshes the peak gauges.
func (c *ControllerV3) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-2 * time.Minute)
	for pid, st := range c.processStates {
		if st.LastUpdate.Before(cutoff) {
			delete(c.processStates, pid)
		}
	}
	c.publishPeak()
}

// GetState returns the current state for a PID (for tests / introspection).
func (c *ControllerV3) GetState(pid int) (wip, price float64, tier Tier, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if st, exists := c.processStates[pid]; exists {
		return st.WIP, st.CurrentPrice, st.CurrentTier, true
	}
	return 0, 0, T2, false
}
