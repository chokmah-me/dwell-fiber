#!/usr/bin/env python3
"""wsl_phase_workload.py -- three-phase synthetic workload to exercise the ACP
phase estimator live on the WSL host.

Run while the V3 daemon is up (any arm), then watch the estimator:
  watch -n1 'curl -s localhost:9090/metrics | grep -E "acp_phase|v3_price |v3_ufm "'

Requires --acp-policy for the phase metric (dwell_fiber_v3_acp_phase:
0=unknown 1=recon 2=learning 3=exploitation); without it, watch v3_price/v3_ufm.

Phases (40 s each, markers printed with timestamps for correlation):
  1. RECON:        paced file enumeration (~150 opens/s, no reads/writes).
                   Expect: PhaseRecon (dampened: alpha x0.4).
  2. LEARNING:     repeated full reads of a fixed 50-file set with shrinking
                   sleep jitter (variance collapses over the phase).
                   Expect: PhaseLearning once CV shrinks over-budget.
  3. EXPLOITATION: steady sustained writes (256 KiB files in a loop).
                   Expect: PhaseExploitation (escalated: alpha x1.8).

All writes go under /tmp/dwell-fiber-phase-test/ and are removed afterwards.
Read-only phase walks /usr (no writes anywhere else).
"""
import os
import sys
import time
import random

PHASE_SECS = 40
WORKDIR = "/tmp/dwell-fiber-phase-test"
ENUM_TARGET_OPENS_PER_SEC = 150


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
    stamp("PHASE 1/3 RECON: paced enumeration (~150 opens/s, no reads/writes)")
    deadline = time.time() + PHASE_SECS
    interval = 1.0 / ENUM_TARGET_OPENS_PER_SEC
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
    stamp(f"  recon done: {count} opens")


def phase_learning():
    stamp("PHASE 2/3 LEARNING: repeated reads of a fixed set, shrinking jitter")
    files = [p for p, _ in zip(iter_files("/usr", 50), range(50))]
    if not files:
        stamp("  no files found; skipping"); return
    deadline = time.time() + PHASE_SECS
    start = time.time()
    rounds = 0
    while time.time() < deadline:
        elapsed = time.time() - start
        # jitter shrinks linearly: 50ms -> 5ms (variance collapse = learning signal)
        jitter = 0.050 * (1 - elapsed / PHASE_SECS) + 0.005
        for path in files:
            try:
                with open(path, "rb") as f:
                    f.read()
            except OSError:
                pass
            time.sleep(random.uniform(0, jitter))
            if time.time() >= deadline:
                break
        rounds += 1
    stamp(f"  learning done: {rounds} read rounds over {len(files)} files")


def phase_exploitation():
    stamp("PHASE 3/3 EXPLOITATION: steady sustained writes")
    os.makedirs(WORKDIR, exist_ok=True)
    deadline = time.time() + PHASE_SECS
    chunk = os.urandom(256 * 1024)
    i = 0
    bytes_written = 0
    while time.time() < deadline:
        path = os.path.join(WORKDIR, f"p3-{i % 64}.dat")
        with open(path, "wb") as f:
            f.write(chunk)
        bytes_written += len(chunk)
        i += 1
        time.sleep(0.05)  # ~5 MB/s steady
    stamp(f"  exploitation done: {i} files, {bytes_written / 1e6:.1f} MB written")


def main():
    stamp("starting 3-phase workload (~2 min total)")
    try:
        phase_recon()
        phase_learning()
        phase_exploitation()
    finally:
        import shutil
        shutil.rmtree(WORKDIR, ignore_errors=True)
    stamp("workload complete; workdir cleaned")


if __name__ == "__main__":
    sys.exit(main())
