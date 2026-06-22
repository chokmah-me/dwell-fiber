#!/usr/bin/env python3
"""
Dwell-Fiber benchmark harness.

Three scenarios, one results table. No frameworks. No magic.

  benign       : extract a 100MB tar of small files (real workload, short dwells)
  attack       : 100 files held open 8s each, random-byte writes (sustained dwell)
  intermittent : 2000 files, open->write 1MB->close each (LockBit-style, <100ms dwell)

For each run we scrape /metrics before+after and report the deltas.
Run ./bin/dwell-fiber-daemon with --enable-enforcement in another terminal first.

Usage:
    python3 test/bench.py --scenario benign
    python3 test/bench.py --scenario attack
    python3 test/bench.py --scenario intermittent
    python3 test/bench.py --scenario both --out BENCHMARKS.md   # benign + attack
    python3 test/bench.py --scenario all  --out BENCHMARKS.md   # + intermittent
"""
import argparse
import os
import random
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request
from pathlib import Path

METRICS_URL = "http://localhost:9090/metrics"
METRIC_KEYS = (
    "dwell_fiber_dwell_time",
    "dwell_fiber_price",
    "dwell_fiber_throttled_count",
    "dwell_fiber_killed_count",
    "dwell_fiber_enforcement_enabled",
    "dwell_fiber_events_total",
    "dwell_fiber_events_filtered_total",
    # V3 (observation-only) rate-based WIP signal; nonzero only when the daemon
    # is run with --use-v3-wip.
    "dwell_fiber_v3_wip",
    "dwell_fiber_v3_price",
    "dwell_fiber_v3_tbw",
    "dwell_fiber_v3_ufm",
)


def scrape():
    """Scrape /metrics, return {key: float} for the gauges we care about."""
    try:
        with urllib.request.urlopen(METRICS_URL, timeout=2) as r:
            text = r.read().decode()
    except Exception as e:
        print(f"[warn] could not scrape {METRICS_URL}: {e}", file=sys.stderr)
        return {}
    out = {}
    for line in text.splitlines():
        if line.startswith("#") or not line.strip():
            continue
        parts = line.split()
        if len(parts) < 2:
            continue
        name = parts[0].split("{")[0]
        if name in METRIC_KEYS:
            try:
                out[name] = float(parts[-1])
            except ValueError:
                pass
    return out


def make_benign_tar(path: Path, n_files: int = 500, size_bytes: int = 200_000):
    """Create a tar of n_files small random files. ~100MB at defaults."""
    with tempfile.TemporaryDirectory() as td:
        td = Path(td)
        for i in range(n_files):
            (td / f"file_{i:05d}.bin").write_bytes(os.urandom(size_bytes))
        with tarfile.open(path, "w") as tf:
            tf.add(td, arcname=".")


def run_benign(workdir: Path) -> dict:
    """Extract a 100MB tar. Many short-dwell file ops."""
    tar_path = workdir / "benign.tar"
    extract_dir = workdir / "benign_out"
    extract_dir.mkdir(exist_ok=True)
    print("[benign] building tar...")
    make_benign_tar(tar_path)

    before = scrape()
    print(f"[benign] before: {before}")
    t0 = time.time()
    print("[benign] extracting...")
    subprocess.run(["tar", "-xf", str(tar_path), "-C", str(extract_dir)], check=True)
    elapsed = time.time() - t0
    # Let the daemon's windowed averaging settle.
    time.sleep(3)
    after = scrape()
    print(f"[benign] after:  {after}")
    return {"scenario": "benign", "elapsed_s": elapsed, "before": before, "after": after}


def run_attack(workdir: Path, n_files: int = 100, hold_s: float = 8.0) -> dict:
    """Open N files, hold each 8s, write random bytes, close.
    This is the sustained-dwell pattern V2 is designed to catch."""
    target = workdir / "attack_out"
    target.mkdir(exist_ok=True)

    before = scrape()
    print(f"[attack] before: {before}")
    print(f"[attack] holding {n_files} files for {hold_s}s each...")
    t0 = time.time()
    for i in range(n_files):
        path = target / f"victim_{i:05d}.dat"
        with open(path, "wb") as f:
            f.write(os.urandom(1024))
            f.flush()
            time.sleep(hold_s)
            f.write(os.urandom(1024))
        # If the daemon killed us, we won't get here.
        if i % 10 == 0:
            print(f"  ... {i}/{n_files}")
    elapsed = time.time() - t0
    time.sleep(3)
    after = scrape()
    print(f"[attack] after:  {after}")
    return {"scenario": "attack", "elapsed_s": elapsed, "before": before, "after": after}


def run_intermittent(workdir: Path, n_files: int = 2000,
                     chunk_bytes: int = 1_048_576) -> dict:
    """Open N files, write a 1MB chunk, close immediately. Repeat.
    This is the LockBit 3.0+ fast-intermittent-encryption pattern: each file
    session is <100ms dwell, well below the 5s budget, so V2.x dwell tracking
    never raises the price. The blind spot, made measurable."""
    target = workdir / "intermittent_out"
    target.mkdir(exist_ok=True)

    before = scrape()
    print(f"[intermittent] before: {before}")
    print(f"[intermittent] writing {n_files} files, {chunk_bytes} bytes each, "
          "open->write->close (no hold)...")
    t0 = time.time()
    for i in range(n_files):
        path = target / f"victim_{i:05d}.dat"
        with open(path, "wb") as f:
            f.write(os.urandom(chunk_bytes))
            f.flush()
        # No sleep: short dwell is the whole point.
        if i % 200 == 0:
            print(f"  ... {i}/{n_files}")
    elapsed = time.time() - t0
    time.sleep(3)
    after = scrape()
    print(f"[intermittent] after:  {after}")
    return {"scenario": "intermittent", "elapsed_s": elapsed,
            "before": before, "after": after}


def fmt_row(r: dict) -> str:
    b = r["before"]
    a = r["after"]

    def delta(k: str) -> str:
        if k not in a:
            return "?"
        return f"{a[k] - b.get(k, 0):+g}"

    return (
        f"| {r['scenario']:<7} | {r['elapsed_s']:>7.1f}s | "
        f"{a.get('dwell_fiber_dwell_time', 0):>5.2f}s | "
        f"{a.get('dwell_fiber_price', 0):>6.3f} | "
        f"{delta('dwell_fiber_throttled_count'):>9} | "
        f"{delta('dwell_fiber_killed_count'):>6} | "
        f"{delta('dwell_fiber_events_total'):>6} | "
        f"{delta('dwell_fiber_events_filtered_total'):>8} | "
        f"{a.get('dwell_fiber_v3_wip', 0):>6.0f} | "
        f"{a.get('dwell_fiber_v3_price', 0):>8.3f} |"
    )


def write_md(results, out: Path):
    # Sample the enforcement flag across every scrape (before+after of each
    # scenario), not just results[0]: a benign run can scrape 0.0 before the
    # flag is observed elsewhere, mislabeling an enforcing daemon as dry-run.
    enf = max(
        (r[phase].get("dwell_fiber_enforcement_enabled", 0)
         for r in results for phase in ("before", "after")),
        default=0,
    )
    enf_label = "ENABLED" if enf >= 1.0 else "DRY-RUN (observation only)"
    n = len(results)
    count_word = {1: "One scenario", 2: "Two scenarios", 3: "Three scenarios"}.get(
        n, f"{n} scenarios")

    lines = [
        "# Dwell-Fiber Benchmarks",
        "",
        f"Enforcement mode during run: **{enf_label}**",
        "",
        f"{count_word} run against a single daemon instance with default config",
        "(`--alpha=0.5 --budget=5.0`).",
        "",
        "| scenario | elapsed | dwell_avg | price | throttled | killed | events | filtered | v3_wip | v3_price |",
        "|----------|--------:|----------:|------:|----------:|-------:|-------:|---------:|-------:|---------:|",
    ]
    for r in results:
        lines.append(fmt_row(r))
    # Per-scenario prose, emitted only for scenarios actually present in this
    # run -- a single-scenario report shouldn't describe rows that aren't there.
    blurbs = {
        "benign": [
            "**Benign** (tar extraction, 500 files, ~200KB each): files are opened,",
            "read/written, closed. Dwells are well below the 5s budget. Price stays",
            "near zero. No enforcement should fire.",
        ],
        "attack": [
            "**Attack** (100 files held 8s each): sustained dwell well above budget.",
            "Each 8s dwell clears the controller's noise filter, so price climbs and",
            "throttle/kill counters increment with `--enable-enforcement --enable-killing`.",
        ],
        "intermittent": [
            "**Intermittent** (2000 files, open->write 1MB->close, no hold): the",
            "LockBit 3.0+ fast-intermittent-encryption pattern. Each file session is",
            "sub-100ms dwell, so it is dropped at the FIRST of two stacked noise",
            "filters -- the in-kernel `<100ms` guard in `bpf/dwell_monitor.bpf.c`,",
            "before the event ever reaches the ring buffer (the userspace",
            "`if dwell < 1*time.Second` filter in `daemon/controller.go` is the",
            "second). The `events`/`filtered` columns count sessions in-kernel,",
            "*before* that filter, so they make the blind spot directly observable:",
            "events climbs into the thousands while filtered tracks it 1:1 and price",
            "stays 0. An armed, kill-enabled daemon rewrites thousands of files with",
            "price=0 / killed=0 -- the V2.x blind spot, root-caused rather than",
            "merely asserted. When the daemon is run with `--use-v3-wip`, the",
            "rate-based V3 detector (observation only) *does* register this: the",
            "`v3_wip`/`v3_price` columns rise while the V2 `price` stays 0 -- the",
            "regression target flipped from blind to detecting.",
        ],
    }
    present = [r["scenario"] for r in results]
    lines += ["", "## What this shows", ""]
    for name in ("benign", "attack", "intermittent"):
        if name in present:
            lines += blurbs[name] + [""]

    if "intermittent" in present:
        lines += [
            "## The measured gap",
            "",
            "V2.x tracks dwell *latency* and drops short sessions as noise (a",
            "<100ms guard in the kernel, then a <1s guard in userspace), so fast",
            "intermittent encryption never registers -- it is not merely",
            "under-budget, it is filtered out at the source. The `events` column is",
            "counted in-kernel before the filter, so it proves the daemon *saw* the",
            "workload (vs. a dead pipeline): thousands of sessions counted, all",
            "filtered, price unmoved. The attack",
            "row (if present) confirms the same build detects and kills long-dwell",
            "activity. The V3.0 WIP-based (rate) architecture is research-in-progress",
            "(unintegrated drafts in `outputs/`, tags v3.0.0-v3.0.2). This",
            "`intermittent` row is the regression baseline any future V3 work must",
            "flip from price~0/killed=0 to detection.",
            "",
        ]
    out.write_text("\n".join(lines))
    print(f"[ok] wrote {out}")


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--scenario",
                   choices=("benign", "attack", "intermittent", "both", "all"),
                   default="both")
    p.add_argument("--out", type=Path, default=Path("BENCHMARKS.md"))
    p.add_argument("--workdir", type=Path, default=Path("/tmp/dwell-fiber-bench"))
    args = p.parse_args()

    args.workdir.mkdir(parents=True, exist_ok=True)
    results = []
    if args.scenario in ("benign", "both", "all"):
        results.append(run_benign(args.workdir))
    if args.scenario in ("attack", "both", "all"):
        results.append(run_attack(args.workdir))
    if args.scenario in ("intermittent", "all"):
        results.append(run_intermittent(args.workdir))

    if not scrape():
        print("[err] daemon /metrics not reachable. Start the daemon first.",
              file=sys.stderr)
        sys.exit(1)

    write_md(results, args.out)


if __name__ == "__main__":
    main()
