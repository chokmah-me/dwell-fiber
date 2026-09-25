# ACP Bridge: Cognitive-Phase Price Policy for V3

**Status:** Implemented, opt-in (`--acp-policy`), unit-tested. **Not validated
against live workloads** -- the multipliers below are starting points.

## Idea

`acp-simulation` models attacker/defender cognition: the `OptimisticACPDefender`
keys its response off attacker knowledge completeness. While the IBLT attacker's
model is still learning (the *cognitive latency window*) it spends only cheap
actions -- deception, honeypot, monitor -- and reserves expensive action for
confirmed compromise. It never pays for `RESTORE_NODE` (cost 6.0) speculatively.

This module ports that *decision logic* into the V3 WIP controller
(`daemon/acp_policy.go`). The daemon cannot observe attacker knowledge, so it
infers a per-PID attacker phase from recent rate behavior and modulates the
ADMM price update per phase instead of using one fixed alpha/budget.

## Concept mapping

| ACP concept | Daemon analogue |
|---|---|
| `attacker_knowledge` completeness | Per-PID observation history depth |
| Cognitive latency window (IBLT activation still diffuse) | `PhaseRecon`: new PID, enumeration signature (high opens/s, ~0 TBW) |
| IBLT model converging (activation concentrating) | `PhaseLearning`: WIP variance shrinking window-over-window while over budget |
| Confident exploitation | `PhaseExploitation`: stable over-budget WIP, CV <= 0.25, sustained 4+ windows |
| Cheap response during latency window (never `RESTORE_NODE`) | Recon: alpha x0.4, budget x1.25 -- observe, don't punish exploration |
| Escalate on confirmed compromise (patch/isolate) | Exploitation: alpha x1.8, budget x0.85 -- reach throttle/kill thresholds faster |

Calibration provenance: `OptimisticACPDefender(acp_strength=0.65)`,
`CognitiveAttacker(decay=0.8)` in acp-simulation. The 8-window history ring
mirrors IBLT activation concentration as the attacker's memory converges.

## Behavior

- Flag: `--acp-policy` (requires `--use-v3-wip`). Off by default; when off the
  controller is bit-identical to before (phase stays `unknown`, multipliers 1.0).
- V3-only. The V2 dwell controller is untouched.
- New metric: `dwell_fiber_v3_acp_phase` (0=unknown, 1=recon, 2=learning,
  3=exploitation for the highest-priced process; -1 when disabled).
- Per-PID phase is kept in `ProcessStateV3.Phase`, readable via `GetPhase`.

## Honest scope

1. This is a heuristic port, not the full IBLT -- the daemon can't run the
   Python sim. Phase signals (enumeration signature, CV thresholds) are
   documented guesses.
2. Multipliers are uncalibrated. The T2 budget problem (STATUS.md: infeasible
   separation on the synthetic bench) applies here too -- the policy changes
   *how fast* prices move, not *where* the budget sits.
3. The ambient open-storm contaminant (~679 opens/s, TBW 0) will read as
   `PhaseRecon` and get dampened pricing -- arguably correct (don't punish
   enumeration), but it also means a real low-and-slow enumerator hides in the
   same bucket. Distinguishing those two is future work.

## Validation path

1. ~~Extend the ACP sim with a dwell-fiber-like workload: fixed vs
   phase-contingent pricing against IBLT attackers, measuring time-to-detect
   vs false-positive cost. This is the Python-side half of the bridge.~~
   **DONE 2026-09-25** -- `acp-simulation/src/acp_simulation/bridge/`
   (`pricing.py`: Go-faithful fixed + phase-contingent controllers;
   `workload.py`: IBLT `CognitiveAttacker` driving per-window I/O signatures
   in closed loop with the defender, benign traces under common random
   numbers; `experiment.py`: paired episode loop + analysis).
   n=200 episodes: detection rate 0.155 -> **0.620** (paired diff +0.465, 95%
   CI [0.39, 0.54], d=0.88, p=9e-21); detection delay median 24.5 -> **9.0**
   windows (paired mean -15.07, CI [-21.61, -9.36], d=-0.89, p<0.0001);
   false positives/episode 0.865 -> **0.605** (paired diff -0.260,
   CI [-0.325, -0.195]). Mechanism: under fixed pricing the IBLT attacker
   learns the throttle boundary and oscillates under the kill threshold;
   1.8x escalation on confirmed exploitation removes that refuge. Caveat:
   the FP reduction is partly recon-dampening slowing the ambient
   contaminant's price climb (it matches the recon signature) -- the policy
   does not distinguish contaminant from real enumeration. Full write-up:
   `acp-simulation/docs/bridge-validation.md`; raw JSON in
   `acp-simulation/results/pricing_validation.json`.
2. `v3_measure.sh` re-run on the WSL host with `--acp-policy`, comparing
   price trajectories on benign / intermittent / ambient scenarios.
   **DONE 2026-09-25** -- `P_b = 0.0`, `P_i = 681.37`; policy-off control arm
   `P_i = 344.20` (same host/workload). ~2x attack-peak lift from the policy
   (ratio 1.98 ~ exploitation multiplier 2.01); gates pass with fixed pricing
   too. One-command runs: `test/wsl_acp_validate.sh [--control]`.
3. Calibrate multipliers (and the phase thresholds) against those runs before
   trusting enforcement. **Partially done** -- the control comparison confirms
   the exploitation multiplier is behaving as designed; the recon/learning
   dampening side was not exercised by the live workload (estimator locked to
   exploitation), so those multipliers remain unvalidated. See
   `docs/v3-calibration.md`.
