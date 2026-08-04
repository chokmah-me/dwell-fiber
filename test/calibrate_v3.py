#!/usr/bin/env python3
"""
V3 threshold calibration harness (offline).

Recommends V3ThrottlePrice / V3KillPrice from measured benign vs intermittent
peak dwell_fiber_v3_price values, or simulates ControllerV3 price dynamics
without root/eBPF/daemon.

Usage:
    python test/calibrate_v3.py --from-peaks --benign-peak 10 --intermittent-peak 100
    python test/calibrate_v3.py --simulate --profile intermittent_attack
    python test/calibrate_v3.py --from-metrics --duration-s 5
    python test/calibrate_v3.py --help
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from typing import Any, Dict, Iterable, List, Optional, Sequence, Tuple

# Starting points from pkg/enforcement/config.go (DefaultConfig). Do not edit Go.
CURRENT_DEFAULTS = {
    "V3ThrottlePrice": 50.0,
    "V3KillPrice": 150.0,
}

# Match daemon/controller_v3.go
DEFAULT_ALPHA = 0.5
DEFAULT_LEAK = 0.9  # defaultLeak
LEAK_SNAP = 0.5

TIER_CONFIGS = {
    "t1": {"omega1": 0.9, "omega2": 0.1, "budget": 3000.0},
    "t1.5": {"omega1": 0.55, "omega2": 0.45, "budget": 1500.0},
    "t2": {"omega1": 0.3, "omega2": 0.7, "budget": 150.0},
}

METRICS_URL = "http://localhost:9090/metrics"
V3_PRICE_METRIC = "dwell_fiber_v3_price"


def leak_price(price: float, leak: float = DEFAULT_LEAK) -> float:
    """Multiplicative decay then snap residual < 0.5 to 0 (ControllerV3.leak)."""
    p = price * leak
    if p < LEAK_SNAP:
        return 0.0
    return p


def update_price_v3(
    price: float,
    wip: float,
    budget: float,
    alpha: float = DEFAULT_ALPHA,
    leak: float = DEFAULT_LEAK,
) -> float:
    """price = max(0, leak(price) + alpha*(wip - budget))."""
    return max(0.0, leak_price(price, leak) + alpha * (wip - budget))


def calculate_wip(tier: str, tbw: float, ufm: float) -> float:
    cfg = TIER_CONFIGS[tier]
    return cfg["omega1"] * tbw + cfg["omega2"] * ufm


def recommend_from_peaks(
    benign_peak: float,
    intermittent_peak: float,
    margin_throttle: float = 0.15,
    kill_ratio: float = 2.0,
) -> Dict[str, Any]:
    """
    Place throttle between peaks; kill above throttle.

    throttle = P_b + M*(P_i - P_b) when P_i > P_b
    kill = throttle * R  (require kill > throttle)
    """
    notes: List[str] = []
    formula = (
        "throttle = P_b + M*(P_i - P_b) when P_i > P_b; "
        "kill = throttle * R (must kill > throttle)"
    )

    if intermittent_peak <= benign_peak:
        # Infeasible separation: cannot place a band between peaks.
        throttle = float("nan")
        kill = float("nan")
        gates = {
            "benign_below_throttle": False,
            "intermittent_clears_throttle": False,
            "kill_above_throttle": False,
        }
        notes.append(
            "Infeasible separation: intermittent_peak <= benign_peak "
            f"({intermittent_peak} <= {benign_peak}). "
            "Cannot place V3ThrottlePrice between scenarios; all gates false. "
            "Re-bench with stronger intermittent load or different tier, "
            "or inspect leak/budget so attack steady-state exceeds benign."
        )
        recommended_throttle = None
        recommended_kill = None
    else:
        throttle = benign_peak + margin_throttle * (
            intermittent_peak - benign_peak
        )
        kill = throttle * kill_ratio
        if kill <= throttle:
            # Enforce kill > throttle even if user passed R <= 1.
            kill = throttle * max(kill_ratio, 1.0 + 1e-9)
            notes.append(
                "kill_ratio did not yield kill > throttle; adjusted to keep "
                "kill strictly above throttle."
            )
        recommended_throttle = throttle
        recommended_kill = kill
        gates = {
            "benign_below_throttle": benign_peak < throttle,
            "intermittent_clears_throttle": intermittent_peak >= throttle,
            "kill_above_throttle": kill > throttle,
        }
        notes.append(
            f"throttle sits {margin_throttle:.0%} of the way from benign peak "
            f"toward intermittent peak; kill = throttle * {kill_ratio}."
        )
        notes.append(
            "GATE A: benign peak below V3ThrottlePrice; "
            "GATE B: intermittent peak reaches/exceeds V3ThrottlePrice; "
            "GATE C: V3KillPrice > V3ThrottlePrice with margin (kill_ratio)."
        )
        notes.append(
            "Starting defaults are V3ThrottlePrice=50, V3KillPrice=150 "
            "(pkg/enforcement/config.go); apply recommended values by hand "
            "on the VM — this harness does not edit Go."
        )

    return {
        "benign_peak": benign_peak,
        "intermittent_peak": intermittent_peak,
        "recommended_V3ThrottlePrice": recommended_throttle,
        "recommended_V3KillPrice": recommended_kill,
        "current_defaults": dict(CURRENT_DEFAULTS),
        "gates": gates,
        "formula": formula,
        "notes": notes,
        "margin_throttle": margin_throttle,
        "kill_ratio": kill_ratio,
    }


def synthetic_profile(name: str) -> List[Dict[str, float]]:
    """
    Synthetic per-window {tbw, ufm} sequences (MB/s, files/s).

    Designed so intermittent_attack peak price > benign_tar peak price under
    default alpha/leak/T2 (budget 300).
    """
    if name == "benign_tar":
        # tar-like: moderate TBW, modest open rate; short burst then idle
        # so leak bleeds price. Under T2, WIP often near/under budget.
        samples: List[Dict[str, float]] = []
        for _ in range(5):
            samples.append({"tbw": 80.0, "ufm": 40.0})  # WIP ≈ 52
        for _ in range(8):
            samples.append({"tbw": 200.0, "ufm": 120.0})  # WIP ≈ 144
        for _ in range(12):
            samples.append({"tbw": 10.0, "ufm": 5.0})  # idle bleed
        return samples

    if name == "intermittent_attack":
        # LockBit-style: many files, 1MB chunks → high UFM + solid TBW, sustained.
        samples = []
        for _ in range(25):
            # WIP = 0.3*120 + 0.7*800 = 36 + 560 = 596; excess ~296 → +148/window
            samples.append({"tbw": 120.0, "ufm": 800.0})
        return samples

    raise ValueError(f"unknown profile: {name!r}")


def load_samples_jsonl(path: str) -> List[Dict[str, float]]:
    samples: List[Dict[str, float]] = []
    with open(path, "r", encoding="utf-8") as f:
        for lineno, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            try:
                obj = json.loads(line)
            except json.JSONDecodeError as e:
                raise SystemExit(f"samples-jsonl line {lineno}: {e}") from e
            if "tbw" not in obj or "ufm" not in obj:
                raise SystemExit(
                    f"samples-jsonl line {lineno}: need keys tbw and ufm"
                )
            samples.append({"tbw": float(obj["tbw"]), "ufm": float(obj["ufm"])})
    if not samples:
        raise SystemExit(f"samples-jsonl empty: {path}")
    return samples


def simulate(
    samples: Sequence[Dict[str, float]],
    tier: str = "t2",
    alpha: float = DEFAULT_ALPHA,
    leak: float = DEFAULT_LEAK,
) -> Dict[str, Any]:
    if tier not in TIER_CONFIGS:
        raise ValueError(f"unknown tier {tier!r}; choose t1|t1.5|t2")
    budget = TIER_CONFIGS[tier]["budget"]
    price = 0.0
    peak = 0.0
    history: List[Dict[str, float]] = []

    for s in samples:
        tbw = float(s["tbw"])
        ufm = float(s["ufm"])
        wip = calculate_wip(tier, tbw, ufm)
        price = update_price_v3(price, wip, budget, alpha=alpha, leak=leak)
        if price > peak:
            peak = price
        history.append(
            {
                "tbw": tbw,
                "ufm": ufm,
                "wip": wip,
                "price": price,
            }
        )

    return {
        "tier": tier,
        "alpha": alpha,
        "leak": leak,
        "budget": budget,
        "peak_price": peak,
        "final_price": price,
        "windows": len(samples),
        "history": history,
    }


def scrape_v3_price(url: str = METRICS_URL, timeout: float = 2.0) -> float:
    """Fetch dwell_fiber_v3_price from Prometheus text exposition."""
    try:
        with urllib.request.urlopen(url, timeout=timeout) as r:
            text = r.read().decode("utf-8", errors="replace")
    except urllib.error.URLError as e:
        raise RuntimeError(f"scrape failed for {url}: {e}") from e
    except TimeoutError as e:
        raise RuntimeError(f"scrape timed out for {url}: {e}") from e
    except OSError as e:
        raise RuntimeError(f"scrape OS error for {url}: {e}") from e

    found = None
    for line in text.splitlines():
        if line.startswith("#") or not line.strip():
            continue
        parts = line.split()
        if len(parts) < 2:
            continue
        name = parts[0].split("{", 1)[0]
        if name == V3_PRICE_METRIC:
            try:
                found = float(parts[-1])
            except ValueError as e:
                raise RuntimeError(
                    f"could not parse {V3_PRICE_METRIC} value from: {line!r}"
                ) from e
    if found is None:
        raise RuntimeError(
            f"{V3_PRICE_METRIC} not found in {url} "
            "(is the daemon running with --use-v3-wip?)"
        )
    return found


def from_metrics(
    duration_s: float = 5.0,
    interval_s: float = 0.5,
    url: str = METRICS_URL,
) -> Dict[str, Any]:
    """Poll metrics for duration_s; return peak and last price."""
    t0 = time.time()
    peak = 0.0
    last = 0.0
    samples = 0
    while True:
        last = scrape_v3_price(url)
        samples += 1
        if last > peak:
            peak = last
        elapsed = time.time() - t0
        if elapsed >= duration_s:
            break
        time.sleep(min(interval_s, max(0.0, duration_s - elapsed)))
    return {
        "metric": V3_PRICE_METRIC,
        "url": url,
        "duration_s": duration_s,
        "samples": samples,
        "peak_price": peak,
        "final_price": last,
    }


def print_from_peaks_report(result: Dict[str, Any]) -> None:
    print("=== V3 threshold calibration (--from-peaks) ===")
    print(f"benign_peak:       {result['benign_peak']}")
    print(f"intermittent_peak: {result['intermittent_peak']}")
    print(
        f"recommended_V3ThrottlePrice: {result['recommended_V3ThrottlePrice']}"
    )
    print(f"recommended_V3KillPrice:     {result['recommended_V3KillPrice']}")
    print(f"current_defaults:  {result['current_defaults']}")
    print(f"gates:             {result['gates']}")
    print(f"formula:           {result['formula']}")
    for n in result["notes"]:
        print(f"note: {n}")
    print("--- JSON ---")
    # JSON-friendly: drop nan, keep None for infeasible
    payload = {
        "benign_peak": result["benign_peak"],
        "intermittent_peak": result["intermittent_peak"],
        "recommended_V3ThrottlePrice": result["recommended_V3ThrottlePrice"],
        "recommended_V3KillPrice": result["recommended_V3KillPrice"],
        "current_defaults": result["current_defaults"],
        "gates": result["gates"],
        "formula": result["formula"],
        "notes": result["notes"],
    }
    print(json.dumps(payload, indent=2, allow_nan=False))


def print_simulate_report(
    result: Dict[str, Any],
    profile: Optional[str] = None,
    samples_path: Optional[str] = None,
) -> None:
    print("=== V3 price simulation (--simulate) ===")
    if profile:
        print(f"profile: {profile}")
    if samples_path:
        print(f"samples_jsonl: {samples_path}")
    print(f"tier={result['tier']} alpha={result['alpha']} leak={result['leak']} "
          f"budget={result['budget']}")
    print(f"windows: {result['windows']}")
    print(f"peak_price:  {result['peak_price']:.6g}")
    print(f"final_price: {result['final_price']:.6g}")
    print("--- JSON ---")
    out = {
        "tier": result["tier"],
        "alpha": result["alpha"],
        "leak": result["leak"],
        "budget": result["budget"],
        "windows": result["windows"],
        "peak_price": result["peak_price"],
        "final_price": result["final_price"],
        "profile": profile,
        "samples_jsonl": samples_path,
    }
    print(json.dumps(out, indent=2))


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="calibrate_v3.py",
        description=(
            "Offline V3ThrottlePrice / V3KillPrice calibration harness for "
            "dwell-fiber. Matches ControllerV3 leak + ADMM update; does not "
            "edit production thresholds."
        ),
    )
    mode = p.add_mutually_exclusive_group(required=False)
    mode.add_argument(
        "--from-peaks",
        action="store_true",
        help="Recommend thresholds from measured benign/intermittent peaks",
    )
    mode.add_argument(
        "--simulate",
        action="store_true",
        help="Simulate V3 price trajectory from a profile or samples JSONL",
    )
    mode.add_argument(
        "--from-metrics",
        action="store_true",
        help=f"Poll {METRICS_URL} for {V3_PRICE_METRIC}",
    )

    p.add_argument("--benign-peak", type=float, default=None, metavar="P_b")
    p.add_argument("--intermittent-peak", type=float, default=None, metavar="P_i")
    p.add_argument(
        "--margin-throttle",
        type=float,
        default=0.15,
        metavar="M",
        help="Fraction of (P_i - P_b) above P_b for throttle (default 0.15)",
    )
    p.add_argument(
        "--kill-ratio",
        type=float,
        default=2.0,
        metavar="R",
        help="kill = throttle * R (default 2.0)",
    )

    p.add_argument(
        "--profile",
        choices=("benign_tar", "intermittent_attack"),
        default=None,
        help="Synthetic sample profile for --simulate",
    )
    p.add_argument(
        "--samples-jsonl",
        default=None,
        metavar="PATH",
        help="JSONL of {tbw,ufm} objects for --simulate",
    )
    p.add_argument(
        "--tier",
        choices=("t1", "t1.5", "t2"),
        default="t2",
        help="Tier weights/budget (default t2)",
    )
    p.add_argument(
        "--alpha",
        type=float,
        default=DEFAULT_ALPHA,
        help=f"ADMM step size (default {DEFAULT_ALPHA})",
    )
    p.add_argument(
        "--leak",
        type=float,
        default=DEFAULT_LEAK,
        help=f"Per-window multiplicative leak (default {DEFAULT_LEAK})",
    )

    p.add_argument(
        "--duration-s",
        type=float,
        default=5.0,
        help="Polling duration for --from-metrics (default 5)",
    )
    p.add_argument(
        "--metrics-url",
        default=METRICS_URL,
        help=f"Metrics URL (default {METRICS_URL})",
    )
    return p


def main(argv: Optional[Sequence[str]] = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    # Default to help-like guidance if no mode chosen (still exit 0 only via --help)
    if not (args.from_peaks or args.simulate or args.from_metrics):
        parser.print_help()
        return 0

    if args.from_peaks:
        if args.benign_peak is None or args.intermittent_peak is None:
            parser.error(
                "--from-peaks requires --benign-peak and --intermittent-peak"
            )
        result = recommend_from_peaks(
            args.benign_peak,
            args.intermittent_peak,
            margin_throttle=args.margin_throttle,
            kill_ratio=args.kill_ratio,
        )
        print_from_peaks_report(result)
        return 0

    if args.simulate:
        samples: List[Dict[str, float]] = []
        if args.samples_jsonl:
            samples.extend(load_samples_jsonl(args.samples_jsonl))
        if args.profile:
            samples.extend(synthetic_profile(args.profile))
        if not samples:
            parser.error(
                "--simulate requires --profile and/or --samples-jsonl"
            )
        result = simulate(
            samples,
            tier=args.tier,
            alpha=args.alpha,
            leak=args.leak,
        )
        print_simulate_report(
            result, profile=args.profile, samples_path=args.samples_jsonl
        )
        return 0

    if args.from_metrics:
        try:
            result = from_metrics(
                duration_s=args.duration_s,
                url=args.metrics_url,
            )
        except RuntimeError as e:
            print(f"error: {e}", file=sys.stderr)
            return 1
        print("=== V3 metrics poll (--from-metrics) ===")
        print(json.dumps(result, indent=2))
        return 0

    return 0


if __name__ == "__main__":
    sys.exit(main())
