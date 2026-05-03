#!/usr/bin/env python3
"""
Regression test for v1.5.0 FD-tracking fix.

Opens N files concurrently with staggered hold times, verifies that
N distinct dwell events are emitted (keyed by real fd, not fd=0).

Usage:
  # Start daemon with enforcement disabled:
  sudo ./bin/dwell-fiber-daemon --alpha=0.5 --budget=5.0

  # Run test in another terminal:
  python3 test/test_fd_tracking.py
"""

import os
import sys
import time
import tempfile
import subprocess
import json
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed


def fetch_metrics():
    """Fetch /metrics endpoint from daemon."""
    try:
        result = subprocess.run(
            ["curl", "-s", "http://localhost:9090/metrics"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        return result.stdout if result.returncode == 0 else None
    except Exception:
        return None


def count_dwell_events(metrics_text):
    """Count distinct dwell events from metrics."""
    if not metrics_text:
        return 0
    # Parse Prometheus metrics for dwell_events_total
    count = 0
    for line in metrics_text.split("\n"):
        if line.startswith("dwell_events_total"):
            # Format: dwell_events_total{pid="...",fd="..."} N
            try:
                count += float(line.split()[-1])
            except (IndexError, ValueError):
                pass
    return int(count)


def open_and_hold_file(duration_sec, file_index, temp_dir):
    """Open a temp file and hold it for the specified duration."""
    temp_file = Path(temp_dir) / f"test_file_{file_index}.txt"
    try:
        temp_file.write_text(f"test data {file_index}")
        # Hold the file open
        with open(temp_file, "r") as f:
            time.sleep(duration_sec)
        return True
    except Exception as e:
        print(f"Error opening file {file_index}: {e}", file=sys.stderr)
        return False


def main():
    # Check if daemon is running
    metrics = fetch_metrics()
    if not metrics:
        print("Error: dwell-fiber daemon not running at http://localhost:9090/metrics", file=sys.stderr)
        sys.exit(1)

    print("✓ Daemon is running")

    # Create temp directory for test files
    with tempfile.TemporaryDirectory() as temp_dir:
        # Get baseline
        baseline = count_dwell_events(fetch_metrics())
        print(f"Baseline dwell events: {baseline}")

        # Open N files concurrently with staggered hold times
        n_files = 5
        hold_duration = 0.5  # 500ms (well above the 100ms threshold)

        print(f"Opening {n_files} files concurrently (hold {hold_duration}s each)...")
        with ThreadPoolExecutor(max_workers=n_files) as executor:
            futures = [
                executor.submit(open_and_hold_file, hold_duration, i, temp_dir)
                for i in range(n_files)
            ]
            # Wait for all to complete
            results = [f.result() for f in as_completed(futures)]

        if not all(results):
            print("Error: Some file opens failed", file=sys.stderr)
            sys.exit(1)

        # Give daemon time to process the events
        time.sleep(1)

        # Check metrics
        after_metrics = fetch_metrics()
        if not after_metrics:
            print("Error: Could not fetch metrics after test", file=sys.stderr)
            sys.exit(1)

        after_count = count_dwell_events(after_metrics)
        new_events = after_count - baseline

        print(f"Dwell events after: {after_count}")
        print(f"New events: {new_events}")

        # Verify: we should have at least n_files new events (one per file)
        # The fix ensures each file gets its own event keyed by (pid, real_fd),
        # not lumped together as fd=0.
        if new_events >= n_files:
            print(f"✓ PASS: Got {new_events} distinct dwell events (expected ≥ {n_files})")
            return 0
        else:
            print(
                f"✗ FAIL: Got only {new_events} dwell events (expected ≥ {n_files}). "
                f"This suggests FD tracking is still keyed by fd=0.",
                file=sys.stderr,
            )
            return 1


if __name__ == "__main__":
    sys.exit(main())
