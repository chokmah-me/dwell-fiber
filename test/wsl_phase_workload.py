#!/usr/bin/env python3
"""wsl_phase_workload.py -- three-phase synthetic workload to exercise the ACP
phase estimator live on the WSL host.

Run while the V3 daemon is up with --acp-policy, then watch:
  watch -n1 'curl -s localhost:9090/metrics | grep -E "acp_phase|v3_price |v3_ufm "'

The phase sequence below is validated against the REAL estimator in
daemon/acp_workload_test.go (synthetic window sequences -> recon -> learning
-> exploitation). T2 numbers (python3 is untiered -> T2): budget=150,
wip = 0.3*tbw + 0.7*ufm.

Phases (markers printed with timestamps for correlation):
  1. RECON (40 s):        paced file enumeration, ~150 opens/s, no writes.
                          WIP ~105 < 150 -> PhaseRecon (dampened x0.4).
  2. LEARNING (60 s):     file churn (create/write 8 KiB/delete) at mean ~280
                          files/s, per-window target jitter shrinking 55% ->
                          35%. WIP ~196 > 150 with collapsing variance ->
                          PhaseLearning. Jitter stays above the 0.25 CV
                          exploitation line, so it should NOT escalate yet.
  3. EXPLOITATION (40 s): same churn, steady at ~280 files/s (+-5%).
                          Stable over-budget -> PhaseExploitation (x1.8).

All churn files live under /tmp/dwell-fiber-phase-test/ and are unlinked as
they are created; the directory is removed afterwards.
"""
import os
import random
import shutil
import sys
import time

P1_SECS = 40
P2_SECS = 60
P3_SECS = 40
WORKDIR = "/tmp/dwell-fiber-phase-test"
ENUM_OPENS_PER_SEC = 150
CHURN_MEAN_PER_SEC = 280
CHUNK = b"x" * (8 * 1024)  # 8 KiB: TBW ~2.2 MB/s at 280/s (above the 1.0 recon cap)


def stamp(msg):
    print(f"[{time.strftime('%H:%M:%S')}] {msg}", flush=True)


def iter_files(root, limit):
    n = 0
    for dirpath, _dirnames, filenames in os.walk(root):
        for fn in filenames:
            yield os.path.join(dirpath, fn)
            n += 1
            if n >= limit:
                return


def phase_recon():
    stamp("PHASE 1/3 RECON: paced enumeration (~150 opens/s, no writes)")
    deadline = time.time() + P1_SECS
    interval = 1.0 / ENUM_OPENS_PER_SEC
    count = 0
    it = iter_files("/usr", 20000)
    while time.time() < deadline:
        t0 = time.time()
        try:
            path = next(it)
        except StopIteration:
            it = iter_files("/usr", 20000)
            continue
        try:
            fd = os.open(path, os.O_RDONLY)
            os.close(fd)
            count += 1
        except OSError:
            pass
        dt = time.time() - t0
        if dt < interval:
            time.sleep(interval - dt)
    stamp(f"  recon done: {count} opens (expect phase=1, dampened price)")


def churn_window(target_ops):
    """One second of create/write/unlink churn; returns ops completed."""
    done = 0
    for i in range(target_ops):
        path = os.path.join(WORKDIR, f"c-{done}-{i}.dat")
        try:
            with open(path, "wb") as f:
                f.write(CHUNK)
            os.unlink(path)
            done += 1
        except OSError:
            pass
    return done


def phase_churn(secs, jitter_start, jitter_end, label, expect):
    stamp(f"{label}: file churn ~{CHURN_MEAN_PER_SEC}/s, "
          f"jitter {jitter_start:.0%} -> {jitter_end:.0%}")
    os.makedirs(WORKDIR, exist_ok=True)
    deadline = time.time() + secs
    start = time.time()
    total = 0
    while time.time() < deadline:
        w0 = time.time()
        elapsed = w0 - start
        j = jitter_start + (jitter_end - jitter_start) * (elapsed / secs)
        target = int(CHURN_MEAN_PER_SEC * (1 + random.uniform(-j, j)))
        total += churn_window(max(target, 1))
        dt = time.time() - w0
        if dt < 1.0:
            time.sleep(1.0 - dt)
    stamp(f"  done: {total} churn ops ({expect})")


def main():
    stamp("starting 3-phase workload (~2.3 min total)")
    try:
        phase_recon()
        phase_churn(P2_SECS, 0.55, 0.35,
                    "PHASE 2/3 LEARNING",
                    "expect phase=2 while variance collapses")
        phase_churn(P3_SECS, 0.05, 0.05,
                    "PHASE 3/3 EXPLOITATION",
                    "expect phase=3, escalated price")
    finally:
        shutil.rmtree(WORKDIR, ignore_errors=True)
    stamp("workload complete; workdir cleaned")


if __name__ == "__main__":
    sys.exit(main())
