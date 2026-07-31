# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Quick Reference

**Module**: `github.com/chokmah-me/dwell-fiber`  
**Language**: Go (daemon) + eBPF C (kernel) + Coq (proofs)  
**Platform**: Ubuntu 25.10+, kernel 5.8+, Go 1.24, clang-20  
**License**: MIT

---

## Build Commands

Targets live in the `Makefile` — read it for the current list (`all`, `bpf`, `coq`,
`daemon`, `test`, `verify`, `clean`, `run`). Non-obvious ordering: the BPF object file
must exist before `make daemon` (the daemon loads it at runtime, it isn't embedded),
and `make run` requires root.

---

## Architecture

**Three-layer design**: Kernel (eBPF) → Userspace (Go) → Formal Proofs (Coq)

### Layer 1: Kernel (`bpf/dwell_monitor.bpf.c`)
- Tracepoints on `sys_openat` and `sys_close`
- Tracks file descriptors by `(pid, fd)` pair
- Measures dwell time (file hold duration) in nanoseconds
- Emits events via ring buffer to userspace

### Layer 2: Userspace Daemon (`daemon/`)
- **Controller** (`daemon/controller.go`): ADMM price update algorithm
  - Formula: `price(t+1) = max(0, price(t) + α*(dwell(t) - budget))`
  - Parameters: `alpha` (step size, 0.5 default), `budget` (5.0 seconds default)
  - Maintains sliding window of recent dwell times (10-event window)
- **Enforcement** (`pkg/enforcement/`): throttle/kill decisions
  - Throttle: reduce CPU via cgroups v2 or nice/renice
  - Kill: SIGTERM (5s grace), then SIGKILL if needed
  - Safety checks: whitelist protected PIDs/processes (systemd, init, etc.)
- **Metrics** (`daemon/metrics.go`): Prometheus + web dashboard on port 9090

### Layer 3: Formal Verification (`coq/`)
- `dwell_stable.v`: core ADMM stability proofs (29/48 complete, 60%)
- `dwell_kernel_resilience.v`: event-loss resilience model
- `dwell_extended.v`: liveness, fairness, attack resistance
- Guarantees: price stays non-negative (Lemma 3), bounded convergence

---

## Critical Ubuntu 25.10 Setup

**Before first build**, run on target system:
```bash
sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm
```

This symlink is **essential** — eBPF compilation will fail without it. If you see `error: #include <asm/types.h>` in clang output, check this symlink first.

---

## Testing

`make test` (unit, fast, no BPF/root needed) and `make verify` (Coq); standard
`go test -v -run TestName ./...` works from `daemon/`.

**Unit tests** (`daemon/controller_test.go`):
- Test ADMM update formula with various price/dwell inputs
- Verify Lemma 3 (price non-negativity)
- Run via `make test` (no root needed)

**Integration tests**:
- `test/daemon/test_burst_loss.go`: simulates 5-event burst loss (Coq-verified max)
- `test/bench.py`: benchmark harness (benign vs attack scenarios)

**CI Pipeline**:
- Push/PR to main triggers: proof verification, eBPF build, daemon build, unit tests, security scan, linting
- Scheduled workflow runs tests weekly (Monday 6am UTC)

---

## Development Workflows

### Adding a Test
1. Create `*_test.go` in target package (e.g., `daemon/new_test.go`)
2. Use `testing.T` interface, no mocking of NewController (panics on duplicate Prometheus registration)
3. If testing controller, construct struct directly: `&Controller{Alpha: 1.5, Budget: 5.0, ...}`
4. Run: `make test`

### Modifying ADMM Algorithm
1. Edit `daemon/controller.go` → `updatePrice()` method
2. Update corresponding unit test in `daemon/controller_test.go`
3. Consider impact on Coq proofs (alpha bounds: 0 < α < 2)
4. Run `make test` then `make verify`

### Changing BPF Tracking Logic
1. Edit `bpf/dwell_monitor.bpf.c`
2. Rebuild: `make bpf`
3. Note: BPF object file must exist before `make daemon` (daemon binary doesn't embed it; loads at runtime)
4. Test with `make run` (requires root + Ubuntu 25.10 + kernel 5.8+)

### Enforcement Policy Changes
1. Edit thresholds in `pkg/enforcement/config.go`
2. Safety rules in `pkg/enforcement/safety.go`
3. Throttle/kill logic in `pkg/enforcement/throttler.go`, `killer.go`
4. Run `make test` for unit coverage
5. Note: enforcement disabled by default (observation mode); use `--enable-enforcement` flag

---

## Module Paths & Imports

Go imports use the full path `github.com/chokmah-me/dwell-fiber`:
```go
import (
    "github.com/chokmah-me/dwell-fiber/pkg/enforcement"
    "github.com/chokmah-me/dwell-fiber/pkg/bpf"
)
```

BPF object file path at runtime: `bpf/dwell_monitor.bpf.o` (relative to binary location or as CLI flag)

---

## Debugging Tips

**BPF won't compile?**
- Check symlink: `ls -l /usr/include/asm`
- Check kernel version: `uname -r` (need 5.8+)
- Check capabilities: `getcap /path/to/bin/clang` (CAP_BPF needed)

**Daemon crashes on startup?**
- Try observation mode (BPF loading fails gracefully, simulator takes over)
- Check Prometheus port 9090 isn't in use
- Verify controller mutex isn't held during shutdown

**Tests fail?**
- Coq proofs failing? Check alpha parameter range (0 < α < 2)
- Go tests panic? If you called `NewController` multiple times in same test binary, that's the cause (Prometheus registration conflict)
- Ring buffer overflow? Implement backpressure (current design drops events under high load)

**Metrics not appearing?**
- Dashboard: `curl http://localhost:9090`
- Metrics endpoint: `curl http://localhost:9090/metrics | grep dwell_fiber`

---

## Notes for Future Sessions

Project status, frozen work, and roadmap are in `STATUS.md`. Key gotcha for task planning:
tests don't require BPF compilation — only the daemon binary needs it; separate test deps from
runtime deps.
