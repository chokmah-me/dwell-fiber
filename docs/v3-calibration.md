# V3 Threshold Calibration

**Purpose:** VM re-tune of `V3ThrottlePrice` and `V3KillPrice` so enforcement
gates match real benign vs intermittent WIP price peaks. Production defaults in
`pkg/enforcement/config.go` are **starting points only** (50 / 150); this doc and
`test/calibrate_v3.py` help you pick numbers without guessing.

See also: `STATUS.md` (Working vs Frozen V3 items), `CHANGELOG.md` threshold note,
and `docs/v3-roadmap.md` calibration status.

---

## Prerequisites

1. Ubuntu host (or VM) with the dwell-fiber daemon built.
2. Run observation first so you can scrape prices without side effects:
   ```bash
   sudo ./bin/dwell-fiber-daemon --use-v3-wip
   ```
3. After thresholds look right, re-run with enforcement (still dry-run until you
   enable killing if desired):
   ```bash
   sudo ./bin/dwell-fiber-daemon --use-v3-wip --v3-enforce
   ```
4. Metrics on `http://localhost:9090/metrics` (especially `dwell_fiber_v3_price`).

No root is required for the **offline** harness (`--from-peaks`, `--simulate`).

---

## Procedure

1. **Bench benign** (tar extract) and record peak `dwell_fiber_v3_price`:
   ```bash
   python3 test/bench.py --scenario benign
   ```
   Note the after/peak `v3_price` (or poll with
   `python3 test/calibrate_v3.py --from-metrics --duration-s 30` during the run).

2. **Bench intermittent** (fast open→write→close) and record peak:
   ```bash
   python3 test/bench.py --scenario intermittent
   ```

3. **Calibrate offline** from the two peaks:
   ```bash
   python3 test/calibrate_v3.py --from-peaks \
     --benign-peak P_b --intermittent-peak P_i
   ```
   Optional: `--margin-throttle 0.15` (default), `--kill-ratio 2.0` (default).

4. **Apply config by hand** — set `V3ThrottlePrice` / `V3KillPrice` in the
   enforcement config (or whatever deployment path you use). This harness does
   **not** edit Go sources.

5. **Re-bench** under `--use-v3-wip` / `--v3-enforce` and confirm the gates below.

### Offline simulation (no daemon)

```bash
python3 test/calibrate_v3.py --simulate --profile benign_tar
python3 test/calibrate_v3.py --simulate --profile intermittent_attack
python3 test/calibrate_v3.py --simulate --samples-jsonl path/to/samples.jsonl --tier t2
```

Simulation mirrors `daemon/controller_v3.go`:

- `price = max(0, leak(price) + α*(WIP − budget))`
- **leak**: multiply by `0.9` (`defaultLeak`), then snap to `0` if result `< 0.5`
- **T2** (default): WIP = `0.3·TBW + 0.7·UFM`, budget `300`
- Tier budgets T1=3000 / T1.5=1500 / T2=300 remain **MVP placeholders** — not
  fully calibrated against real ransomware samples (see STATUS.md Frozen).

---

## Regression gates (GATE A / B / C)

| Gate | Requirement |
|------|-------------|
| **GATE A** | Benign peak `v3_price` stays **below** `V3ThrottlePrice` |
| **GATE B** | Intermittent peak **reaches or exceeds** `V3ThrottlePrice` |
| **GATE C** | `V3KillPrice` **>** `V3ThrottlePrice` with margin (default kill = throttle × 2) |

Formula used by the harness when `P_i > P_b`:

```
throttle = P_b + M * (P_i - P_b)     # M default 0.15
kill     = throttle * R              # R default 2.0
```

If `P_i ≤ P_b`, separation is infeasible: all gates report false and notes
explain that the attack did not out-price benign (re-bench or re-check leak /
tier / load).

---

## Notes

- **Words for search / agents:** `V3ThrottlePrice`, `V3KillPrice`, intermittent,
  benign, leak, GATE — these are the knobs and gates this doc owns.
- Tier budgets and UFM (opens/s proxy vs true unique-inode) are still MVP; do not
  invent CO-RE/UFM claims. True inode UFM remains Frozen in STATUS.md.
- Unit check: `python test/test_calibrate_v3.py` (stdlib only, exit 0).
