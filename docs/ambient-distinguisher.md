# Ambient-vs-enumeration distinguisher — design

**Status:** specified 2026-09-27; prototype in `daemon/ambient.go` (observe-only).
**Problem (workplan item 4):** the ACP policy's recon dampening (α×0.4) also
slows the ambient contaminant's price climb, because ambient open storms match
the recon signature (UFM ≥ 50/s, TBW ≈ 0). The policy cannot tell real attacker
enumeration from an ambient open storm — the sim's FP win is partly
attributed to dampening the contaminant, and live calibration's `P_b` is
polluted by it.

## Why per-PID estimation cannot do this

The phase estimator (`daemon/acp_policy.go`) keeps an 8-window ring **per
PID**. Ambient bursters are short-lived: each ~30 s burst arrives on a fresh
PID (`2382`, `2383`, `2747`, …) that never survives long enough to progress
past recon. Per-PID state therefore sees every ambient burst as "a new
process doing enumeration" — indistinguishable from a real attacker's first
recon windows. The distinguishing information only exists **across** PIDs:
the metronomic cadence and the PID churn.

## Signals

| Signal | Ambient | Real recon | Cost |
|---|---|---|---|
| **Burst periodicity** (system-wide inter-arrival of recon-signature bursts) | metronomic (~30 s, low jitter) | irregular, attacker-driven | cheap (timestamps already in handler) |
| **PID churn** (distinct burst PIDs / total bursts, 5-min window) | ~1.0 — fresh PID per burst | low — one PID persists through recon→learning→exploitation | cheap (small ring) |
| Parent provenance (PPID / parent comm) | system services (cron, apt, cloud-init) | user shell / infection vector | medium — needs BPF `real_parent` walk or racy `/proc` read; **future** |
| Opened paths | same system dirs every burst | user data, expanding | expensive — filename capture in BPF; **future** |

The prototype uses periodicity + churn only. Provenance and paths are
documented future work, not this change.

## Design: `AmbientDetector`

New file `daemon/ambient.go`. Fed from `ControllerV3.HandleWIPSample` on the
**recon signature** (`ufm >= 50/s && tbw < 1.0 MB/s` — the estimator's
constants, reused so the detector sees exactly what the policy dampens),
independent of whether `--acp-policy` is on.

- `ObserveBurst(pid, comm, now)`: appends to a bounded ring (last 32 bursts);
  records `firstSeen[pid]`.
- `StormActive() bool`: true when **both** hold over the recent ring:
  1. **Metronome**: ≥ 4 bursts, inter-arrival CV ≤ 0.25, median period in
     [10 s, 120 s].
  2. **Churn**: distinct burst PIDs / total bursts ≥ 0.75 over the last
     5 min (ambient puts each burst on a fresh short-lived PID; a
     persistent process bursting repeatedly scores low and stays with the
     per-PID estimator).
- Decay: storm clears after 3× the median period with no burst.
- All thresholds are starting points, documented in code.

### Outputs (observe-only — no pricing change)

- `dwell_fiber_v3_ambient_storm` gauge (0/1): ambient-storm regime active.
- `dwell_fiber_v3_ambient_bursts_total` counter: bursts labeled ambient.
- Intended uses: (a) dashboard visibility; (b) **clean `P_b`** — the
  calibration harness can exclude ambient-labeled windows instead of the
  current whole-log `max(price=)` hack; (c) evidence for the threshold
  lock-in decision (true ambient ceiling vs throttle margin).

### Why the conjunction matters

A single persistent PID bursting metronomically fails the churn condition →
not a storm → per-PID logic handles it (sustained pressure still escalates
to exploitation — correct, since a persistent high-pressure process *should*
be priced). The detector fires only for the churn+metronome combination that
per-PID state is blind to.

## Adversarial analysis (honest scope)

- **Mimicry**: an attacker doing slow, periodic, short-lived enumeration
  bursts would be labeled ambient and dampened. This is accepted: such an
  attacker never progresses, hence never does damage — the moment it writes
  (TBW ≥ 1) or sustains pressure on one PID, the signature breaks and
  per-PID escalation takes over. The detector is a *measurement aid*, not
  the security boundary.
- **Evasion by jitter**: randomizing burst intervals defeats the metronome
  check → falls back to today's behavior (dampened as recon). No worse than
  status quo.
- The detector never *reduces* scrutiny of a progressing PID: phase
  escalation is unchanged.

## Future work

1. Parent-provenance capture at window-create (`real_parent` walk with
   `CORE_READ_PROBE`, same pattern as the verifier fix).
2. Harness integration: `v3_measure.sh` reads the storm gauge to compute a
   real ambient ceiling per window.
3. Revisit thresholds against labeled live data once the ambient source
   (workplan item 1) is identified.
