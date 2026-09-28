package main

// Ambient-vs-enumeration distinguisher (prototype).
//
// Problem: the ACP policy dampens anything matching the recon signature
// (ufm >= reconOpensPerSec && tbw < reconTBWCap), and the ambient open storms
// match it. The per-PID phase estimator cannot tell them apart because
// ambient bursters are short-lived: each ~30 s burst arrives on a fresh PID
// that never lives long enough to progress past recon. The distinguishing
// information only exists *across* PIDs: metronomic cadence + PID churn.
//
// This detector is system-level and observe-only: it never changes pricing.
// It exports a storm gauge and a burst counter so dashboards and the
// calibration harness can see / exclude the ambient regime. See
// docs/ambient-distinguisher.md for the full design and adversarial analysis.
//
// All thresholds are documented starting points.

import (
	"math"
	"time"
)

const (
	ambientRingN       = 32               // recent bursts kept
	ambientMinBursts   = 4                // bursts before a storm is possible
	ambientMaxInterCV  = 0.25             // inter-arrival CV ceiling for "metronomic"
	ambientMinPeriod   = 10 * time.Second // plausible infrastructure cadence floor
	ambientMaxPeriod   = 120 * time.Second
	ambientChurnWindow = 5 * time.Minute  // lookback for the churn ratio
	ambientChurnFrac   = 0.75             // distinct-PID/burst ratio for "high churn"
)

type ambientBurst struct {
	t   time.Time
	pid int
}

// AmbientDetector labels system-wide recon-signature burst regimes.
type AmbientDetector struct {
	bursts      []ambientBurst
	storm       bool
	lastPeriod  time.Duration
	totalBursts int64
}

// NewAmbientDetector returns an empty detector.
func NewAmbientDetector() *AmbientDetector {
	return &AmbientDetector{}
}

// ObserveBurst records one recon-signature burst for a PID. now is a parameter
// (not time.Now) so tests can drive synthetic timelines.
func (d *AmbientDetector) ObserveBurst(pid int, now time.Time) {
	if len(d.bursts) == ambientRingN {
		copy(d.bursts, d.bursts[1:])
		d.bursts = d.bursts[:ambientRingN-1]
	}
	d.bursts = append(d.bursts, ambientBurst{t: now, pid: pid})
	d.totalBursts++
	d.recompute(now)
}

// Poll advances time without a burst (handles storm decay). Callers that only
// observe bursts should still poll on a tick so the gauge clears.
func (d *AmbientDetector) Poll(now time.Time) {
	if d.storm && len(d.bursts) > 0 && now.Sub(d.bursts[len(d.bursts)-1].t) > 3*d.lastPeriod {
		d.storm = false
	}
}

// StormActive reports whether the ambient-storm regime is currently active.
func (d *AmbientDetector) StormActive() bool { return d.storm }

// TotalBursts returns the number of recon-signature bursts observed.
func (d *AmbientDetector) TotalBursts() int64 { return d.totalBursts }

// recompute re-evaluates the storm conditions after each burst. Both
// conditions are computed over the same recent window (last
// ambientChurnWindow); bursts older than that neither establish nor sustain
// a storm.
func (d *AmbientDetector) recompute(now time.Time) {
	cutoff := now.Add(-ambientChurnWindow)
	var recent []ambientBurst
	for _, b := range d.bursts {
		if !b.t.Before(cutoff) {
			recent = append(recent, b)
		}
	}
	if len(recent) < ambientMinBursts {
		d.storm = false
		return
	}
	// Metronome: inter-arrival times of the recent bursts.
	n := len(recent)
	diffs := make([]float64, 0, n-1)
	for i := 1; i < n; i++ {
		diffs = append(diffs, recent[i].t.Sub(recent[i-1].t).Seconds())
	}
	median, cv := medianCV(diffs)
	metronome := cv <= ambientMaxInterCV &&
		median >= ambientMinPeriod.Seconds() && median <= ambientMaxPeriod.Seconds()

	// Churn: distinct burst PIDs / total bursts over the window. Ambient
	// storms put each burst on a fresh short-lived PID (ratio ~1); a
	// persistent process bursting repeatedly scores low and stays with the
	// per-PID estimator, which is the correct owner for it.
	distinct := make(map[int]struct{}, len(recent))
	for _, b := range recent {
		distinct[b.pid] = struct{}{}
	}
	churn := float64(len(distinct))/float64(len(recent)) >= ambientChurnFrac

	if metronome {
		d.lastPeriod = time.Duration(median * float64(time.Second))
	}
	d.storm = metronome && churn
}

// medianCV returns the median and coefficient of variation of xs.
// Empty input yields (0, +Inf): no signal, never a false "metronomic".
func medianCV(xs []float64) (median, cv float64) {
	n := len(xs)
	if n == 0 {
		return 0, math.Inf(1)
	}
	sorted := make([]float64, n)
	copy(sorted, xs)
	for i := 1; i < n; i++ { // insertion sort; n is tiny
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if n%2 == 1 {
		median = sorted[n/2]
	} else {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(n)
	if mean == 0 {
		return median, math.Inf(1)
	}
	var sq float64
	for _, x := range xs {
		dd := x - mean
		sq += dd * dd
	}
	return median, math.Sqrt(sq/float64(n)) / mean
}
