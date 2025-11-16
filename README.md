# 🛡️ Dwell-Fiber

**Ransomware Defense Through Proven-Stable Economic Enforcement**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Ubuntu 25.10](https://img.shields.io/badge/Ubuntu-25.10-orange.svg)](https://ubuntu.com/)
[![Coq 8.20+](https://img.shields.io/badge/Coq-8.20%2B-blue.svg)](https://coq.inria.fr/)
[![Version: v1.3.0](https://img.shields.io/badge/Version-v1.3.0-green.svg)](https://github.com/dyb5784/dwell-fiber/releases/tag/v1.3.0)
[![Build: Proofs WIP](https://img.shields.io/badge/Proofs-WIP-yellow.svg)](https://github.com/dyb5784/dwell-fiber)

## ⚠️ v1.3.0 - Enforcement Live & Tested

**This release includes full end-to-end enforcement capabilities.** Enforcement is OFF by default for safety, but can be enabled with `--enable-enforcement`.

```bash
# Observation mode (safe default - just monitor):
sudo ./bin/dwell-fiber-daemon

# Enable enforcement if desired:
sudo ./bin/dwell-fiber-daemon --enable-enforcement --enable-killing
```

**Note**: Coq formal proofs are under active development (type system fixes in progress). See [CHANGELOG.md](CHANGELOG.md) for details.

## 📖 Documentation

**New to Dwell-Fiber?** Start with the **[User Guide](USER_GUIDE.md)** — a plain-English explanation for non-technical users covering setup, dashboard interpretation, and real-world usage.

**Looking for technical details?** See sections below or the [architecture docs](docs/architecture.md).

---

# Dwell-Fiber V3: Adaptive I/O Pricing Ransomware Defense

Dwell-Fiber is a **formally-verified eBPF-based ransomware defense system** that enforces economic costs on suspicious file access patterns using **Weighted I/O Pressure (WIP)** pricing and **ADMM optimization** with proven stability guarantees.

## Core Innovation: WIP Metric

V3 replaces the flawed dwell-time metric (bypassed by sub-second ransomware access) with **rate-based signals** tracked over 1-second windows:

- **TBW** (Total Bytes Written): Volume in MB/s
- **UFM** (Unique Files Modified): Scattergun count in Files/s

Weighted I/O Pressure combines them:
```
WIP(t) = ω₁·TBW(t) + ω₂·UFM(t)
```

Dynamic weights (ω₁, ω₂) adapt via **Trust Classification Module (TCM)** based on process behavior:

| Tier | Profile         | ω₁  | ω₂  | Budget | Detection Criteria |
|------|----------------|-----|-----|--------|-------------------|
| T1   | Backups        | 0.9 | 0.1 | 12000  | TBW ≥ 10k MB/s OR UFM ≤ 1k |
| T1.5 | Dev Builds     |0.55 |0.45 | 8000   | UFM ≥ 20k & TBW ≥ 500 |
| T2   | Untrusted      |0.3  |0.7  | 4000   | TBW < 10 & UFM ≥ 1k |

## System Architecture

Three-layer design: Kernel (eBPF) → Userspace (Go Daemon) → Formal (Coq).

- **Kernel**: kprobe/vfs_write aggregates TBW/UFM per PID per 1s window, emits ringbuf events.
- **Userspace**: TCM classifies tier, ADMM updates price: π(t+1) = max(0, π(t) + α·(WIP - B_tier)).
- **Formal**: Coq proves WIP convexity, price boundedness under switches, Lyapunov drift.

See [ARCHITECTURE.md](ARCHITECTURE.md) for full details.

## V3 Transition from V0

V0 used dwell time (file open duration), defeated by intermittent ransomware. V3 uses rate-based WIP to detect malicious patterns without latency dependence.

Migration details: [V3_MIGRATION.md](V3_MIGRATION.md).

## Build & Run

Prerequisites:
```bash
sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm
sudo apt-get install clang libbpf-dev coq golang-1.24
```

Build order:
```bash
make bpf      # eBPF object
make verify   # Coq proofs (<1s)
make daemon   # Go binary
```

Run:
```bash
sudo ./bin/dwell-fiber
# Metrics: http://localhost:9090
```

## Documentation

- [ARCHITECTURE.md](ARCHITECTURE.md) – Three-layer design and data flow
- [V3_MIGRATION.md](V3_MIGRATION.md) – V0→V3 pivot: dwell to WIP
- [FORMAL_VERIFICATION.md](FORMAL_VERIFICATION.md) – Coq proof structure and lemmas
- [CHANGELOG.md](CHANGELOG.md) – Version history and changes
- [DEV-NOTES.md](DEV-NOTES.md) – Development notes and technical debt

## Key Parameters

- `alpha`: ADMM step size (0.6, proven stable 0 < α < 2)
- `Δt`: Window size (1.0s)
- `throttle_price`: Threshold for throttling (500)
- `kill_price`: Threshold for termination (1000)

## Testing

- Unit: `go test ./...`
- Formal: `make verify`
- Integration: `sudo test/run_e2e.sh`

## License

[License text here]

## Contributing

See CONTRIBUTING.md for guidelines.

---

# Overview

Dwell-Fiber is a formally-verified eBPF-based system that prevents ransomware by enforcing economic costs on file access patterns.

### Key Innovation

Traditional ransomware detection relies on behavioral signatures that can be evaded. Dwell-Fiber takes a different approach:

1. **Monitor** file "dwell time" (how long processes keep files open)
2. **Price** file access using ADMM optimization (proven stable)
3. **Enforce** via throttling/termination when prices are high
4. **Guarantee** mathematical properties via Coq proofs

## Features

- 🛡️ **Real-time Protection**: eBPF-based monitoring of file dwell times
- 📊 **Economic Enforcement**: ADMM-based pricing that adapts to process behavior
- ✅ **Formally Verified**: Coq proofs guarantee system stability
- 🚀 **Low Overhead**: Sub-millisecond latency impact
- 📈 **Observable**: Built-in Prometheus metrics and web UI
- 👥 **User-Friendly**: Safe-by-default (observation mode), explicit `--enable-enforcement` to activate
- 🔒 **Cross-Platform**: Platform-specific build tags for Linux/Unix
- ⚡ **Enforcement Live**: Throttling via cgroups v2, process killing with safety checks
- 🧪 **Tested Scenarios**: 4 workload modes including attack simulation

## Quick Links

| Audience | Resource |
|----------|----------|
| **End Users** | [User Guide](USER_GUIDE.md) — 5-minute setup, no jargon |
| **Developers** | [Architecture Docs](docs/architecture.md) — system design |
| **Researchers** | [Stability Proofs](coq/dwell_stable.v) — formal verification |
| **DevOps** | [Deployment Guide](docs/making-of.md) — systemd setup, monitoring |
| **Release Notes** | [v1.3.0 Changelog](CHANGELOG.md) — bug fixes, security updates |

## Mathematical Guarantees

The system is **proven** to satisfy (see `coq/dwell_stable.v`):

✅ **Convergence**: Price reaches optimal value in finite time  
✅ **Constraint Satisfaction**: Dwell time eventually stays ≤ 5 seconds  
✅ **Boundedness**: Price never goes negative or infinite  
✅ **Stability**: No oscillations or divergence  
✅ **Parameter Range**: Works for any step size 0 < α < 2

## Quick Start (5 Minutes)

### Prerequisites (Ubuntu 25.10)

```bash
sudo apt-get update
sudo apt-get upgrade -y

sudo apt-get install -y \
    clang llvm libbpf-dev \
    golang-go coq make git

# Critical: Fix Ubuntu 25.10 asm symlink
sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm
```

### Build

```bash
git clone https://github.com/dyb5784/dwell-fiber.git
cd dwell-fiber

# Build all components
make all

# Verify mathematical proofs (180ms)
make verify
```

### Run (Observation Mode — Safe Default)

```bash
# Start daemon in observation-only mode (no enforcement)
sudo ./bin/dwell-fiber-daemon --alpha=0.5 --budget=5.0

# In another terminal, check status
curl http://localhost:9090/health
curl http://localhost:9090/metrics

# Or open web UI
firefox http://localhost:9090
```

**See [USER_GUIDE.md](USER_GUIDE.md) for:**
- Dashboard interpretation
- Command-line options
- Real-world usage scenarios
- Troubleshooting

## Repository Structure

```
dwell-fiber/
├── bpf/                      # eBPF kernel programs
│   ├── dwell_monitor.bpf.c   # File dwell time tracker
│   └── Makefile
├── coq/                      # Formal proofs
│   ├── dwell_stable.v        # Stability proof (ADMM)
│   └── Makefile
├── daemon/                   # Control daemon (Go)
│   ├── main.go              # Entry point
│   ├── controller.go        # ADMM implementation
│   └── metrics.go           # HTTP metrics server
├── pkg/                     # Reusable packages
├── scripts/                 # Helper scripts
├── USER_GUIDE.md           # End-user guide ⭐ START HERE
├── Makefile                 # Root build system
├── go.mod                   # Go dependencies
└── README.md               # This file
```

## ADMM Algorithm

The controller implements the **Alternating Direction Method of Multipliers**:

```
price(t+1) = max(0, price(t) + α × (dwell(t) - budget))
```

**Where:**
- `α = 0.5` (step size, proven stable for 0 < α < 2)
- `budget = 5 seconds` (configurable via `--budget`)
- `dwell(t)` = measured file dwell time at iteration t

**Why ADMM?**
1. **Provably Convergent**: Lyapunov theory guarantees convergence
2. **Distributed**: Each process has independent pricing
3. **Robust**: Handles noisy measurements gracefully
4. **Fast**: Converges in ~20 iterations

See the [stability proof explanation](docs/stability-proof.md) for details.

## Performance

Observed on Ubuntu 25.10 (kernel 6.17), Go 1.25, VM environment:

- eBPF attach and ring buffer
  - Tracepoints: sys_enter_openat/sys_enter_close
  - Events < 0.1s dwell are filtered as noise (reduces load)
  - Ring buffer remained stable during workload-generator runs

- End-to-end decision latency
  - Enforcement is triggered on file close (by design)
  - Close event → decision → metrics update occurs within milliseconds of the close
  - For continuous mode (-mode=2), the action/logs appear when the file closes

- Controller overhead
  - ADMM update and decision logic are O(1) per event
  - Observed CPU overhead negligible in steady state (~sub-1% on the VM)
  - Daemon memory footprint ~50–80 MB (typical Go HTTP server + metrics)

- Enforcement costs
  - Throttling: single cgroups v2 write per PID per dedup window (default 10s)
  - Kill path: SIGTERM (graceful) with fallback to SIGKILL if still alive

- Metrics and UI
  - Prometheus registry exposed at /metrics via promhttp
  - Safe to scrape at 1s intervals; web UI auto-refreshes every second

**Note:** Exact numbers vary with workload and hardware. The above reflects indicative behavior from validation on an Ubuntu 25.10 VM.

## Security Considerations

⚠️ **This system requires root/CAP_BPF privileges**

**Why?**
- eBPF programs must be loaded into the kernel
- Enforcement requires killing/throttling processes
- Reading from kernel ring buffers requires privileges

**Best Practices:**
- Run daemon as systemd service with minimal privileges
- Use AppArmor/SELinux profiles to restrict daemon
- Monitor daemon logs for anomalies
- Limit enforcement to specific users/groups
- **Start in observation mode** (no `--enable-enforcement` flag) to learn your workload

## Current Status

**v1.3.0 Release: Enforcement Live & Tested**

Implemented:
- ✅ Real BPF monitoring (attached to sys_enter_openat/sys_enter_close) with ring-buffer events
- ✅ eBPF inode/FD tracking fixed (ISSUE #1) — now supports multiple files per PID
- ✅ Enforcement OFF by default (safe-by-default model) — explicit `--enable-enforcement` required
- ✅ Platform-specific Unix imports (ISSUE #2) — proper cross-platform support
- ✅ Stale BPF map cleanup (ISSUE #4) — prevents memory leaks from crashed processes
- ✅ 100ms noise filter at eBPF close handler (ISSUE #5)
- ✅ Enforcement engine (opt-in):
  - Throttling via cgroups v2 (default 20% CPU quota)
  - Killing on critical dwell (default 15s+), graceful SIGTERM then SIGKILL
- ✅ Safety controls: protected process list (systemd, init, sshd, NetworkManager, gdm, Xorg, wayland) and self-protection
- ✅ ADMM controller with α=0.5, budget=5s; responsive rolling average and price updates
- ✅ Metrics and UI:
  - Prometheus registry at /metrics (throttled_count, killed_count, enforcement_enabled, dwell, price)
  - Web UI at / with live status, dwell, price, and enforcement counts
- ✅ Workload generator with 3 modes:
  - Mode 1: Full test (quick, immediate events)
  - Mode 2: Continuous (emits on close; use shorter duration for faster feedback)
  - Mode 3: Attack simulation (7s/10s throttled, 15s killed)
- ✅ Coq proofs compile (stability)
- ✅ End-user guide with setup/usage/troubleshooting
- ✅ Updated CI/CD for Ubuntu 25.10 + Go 1.24

In Progress:
- 🚧 Mid‑dwell enforcement (act before file close)
- 🚧 Performance profiling (BPF overhead, ring-buffer drop rate, controller CPU/mem)
- 🚧 Systemd service packaging and runbook
- 🚧 CI/integration tests (GitHub Actions E2E)

Planned:
- 📋 Threshold and policy tuning via CLI/config (per‑user/per‑cmd policies)
- 📋 Adaptive/dynamic thresholds and anomaly detection
- 📋 Distributed deployment guidance and hardening
- 📋 Additional Coq proofs (liveness, fairness, attack resistance)

## Contributing

Contributions welcome! Areas of interest:

- eBPF optimizations
- Additional Coq proofs (liveness, fairness)
- Enforcement strategies
- Testing infrastructure
- Documentation

Please open an issue before starting major work.

## License

MIT License - See [LICENSE](LICENSE) file

## Citation

If you use Dwell-Fiber in research, please cite:

```bibtex
@software{dwell_fiber_2025,
  title = {Dwell-Fiber: Formally-Verified Ransomware Defense},
  author = {dyb},
  year = {2025},
  url = {https://github.com/dyb5784/dwell-fiber}
}
```

## Acknowledgments

I drew on optimization-decomposition ideas for network architectures (notably Doyle & Chiang, 2007) and the broader NUM literature, and integrated them with formal verification techniques. An associated proprietary Universal Decomposition Canon artifact (a distilled 'Thoughtbase) was used to generate decomposition heuristics and sigil library used for sigil remapping.

A Thoughtbase is a structured, retrievable, and interconnected mesh of thoughts about information. The Insight Cluster is the fundamental, indivisible unit of a Thoughtbase. It is a cognitively potent node that encapsulates a single, distilled "thought," forged from the raw chaos of unstructured data. TBIC pre-compute meaning and relationships. They enable AI to recognize patterns, contrast ideas, and generate nuanced strategies by giving it a deep, conceptual map of knowledge, turning data into AI-native actionable assets.

Key influences:

- Doyle & Chiang (2007) — "Layering as optimization decomposition" (see docs/overview.md)
- Dave Aitel (December 2016) — "Dwell Time" talk at https://youtu.be/PmabStfUdPk
- Daniel Miessler Unsupervised Learning Newsletter (2023-) https://newsletter.danielmiessler.com/

## References

- [eBPF Documentation](https://ebpf.io/)
- [Coq Proof Assistant](https://coq.inria.fr/)
- [libbpf](https://github.com/libbpf/libbpf)

---

**Status:** Active Development  
**Latest Release:** v1.3.0 (November 6, 2025)  
**Last Updated:** November 6, 2025  
**Maintainer:** [@dyb5784](https://github.com/dyb5784)
