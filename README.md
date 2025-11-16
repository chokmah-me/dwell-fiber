# 🛡️ Dwell-Fiber

**Ransomware Defense Through Proven-Stable Economic Enforcement**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Ubuntu 25.10](https://img.shields.io/badge/Ubuntu-25.10-orange.svg)](https://ubuntu.com/)
[![Coq 8.20+](https://img.shields.io/badge/Coq-8.20%2B-blue.svg)](https://coq.inria.fr/)
[![Version: v1.3.0](https://img.shields.io/badge/Version-v1.3.0-green.svg)](https://github.com/dyb5784/dwell-fiber/releases/tag/v1.3.0)
[![Build: V3.0 In Progress](https://img.shields.io/badge/V3.0-In%20Progress-yellow.svg)](https://github.com/dyb5784/dwell-fiber)

## ⚠️ Important: V2.x vs V3.0 Status

**Current Production**: V2.x (dwell-time based) - **Fully functional**  
**In Development**: V3.0 (WIP-based) - **Components in draft, not integrated**

```
✅ V2.x (Production): 
   - Code: daemon/, bpf/ directories
   - Status: Enforcement live, metrics working, dashboard functional
   - Limitation: Vulnerable to intermittent ransomware (LockBit pattern)

⚠️ V3.0 (Development):
   - Code: Draft components in outputs/ directory
   - Status: Architecture designed, components drafted, NOT integrated
   - Timeline: 21-33 hours of work remaining for full integration
```

**See [V3_MIGRATION_STATUS.md](V3_MIGRATION_STATUS.md) for complete status.**

---

## What is Dwell-Fiber?

Dwell-Fiber prevents ransomware by enforcing economic costs on file access patterns using ADMM (Alternating Direction Method of Multipliers) optimization.

### V2.x: Dwell-Time Based (Current Production)

Monitors how long processes keep files open:
- **Metric**: File dwell time (seconds between open/close)
- **Budget**: 5 seconds
- **Enforcement**: Throttle at 5s+, kill at 15s+
- **Detection**: Ransomware holding files open during encryption

**Known Vulnerability**: Modern ransomware (LockBit 3.0+) uses intermittent encryption - opens, encrypts 1MB, immediately closes (< 100ms). V2.x cannot detect this pattern.

### V3.0: Weighted I/O Pressure (In Development)

Replaces latency-based dwell time with rate-based signals:
- **TBW** (Total Bytes Written): MB/s over 1-second windows
- **UFM** (Unique Files Modified): Files/s over 1-second windows
- **WIP = ω₁·TBW + ω₂·UFM**: Weighted combination with adaptive weights

**Why V3.0?**: Detects high-velocity I/O patterns regardless of session duration. See [V3_PIVOT_RESEARCH_DOSSIER.md](V3_PIVOT_RESEARCH_DOSSIER.md) for empirical analysis.

---

## Quick Start (V2.x - Production)

### Prerequisites (Ubuntu 25.10)

```bash
sudo apt-get update
sudo apt-get install -y clang llvm libbpf-dev golang-go coq make git

# CRITICAL: Fix Ubuntu 25.10 asm symlink
sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm
```

### Build

```bash
git clone https://github.com/dyb5784/dwell-fiber.git
cd dwell-fiber

# Build all components
make all

# Verify mathematical proofs (⚠️ currently has compilation errors)
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

**Enable Enforcement** (use with caution):
```bash
sudo ./bin/dwell-fiber-daemon --enable-enforcement --enable-killing
```

---

## Features

### V2.x (Production)
- 🛡️ **Real-time Protection**: eBPF-based monitoring of file dwell times
- 📊 **Economic Enforcement**: ADMM-based pricing that adapts to process behavior
- ✅ **Formally Verified**: Coq proofs guarantee system stability (⚠️ proofs have compilation errors)
- 🚀 **Low Overhead**: Sub-millisecond latency impact
- 📈 **Observable**: Built-in Prometheus metrics and web UI
- 👥 **User-Friendly**: Safe-by-default (observation mode), explicit `--enable-enforcement` to activate
- ⚡ **Enforcement Live**: Throttling via cgroups v2, process killing with safety checks
- 🧪 **Tested Scenarios**: 4 workload modes including attack simulation

### V3.0 (Planned)
- 🎯 **Adaptive Tier Classification**: TCM module classifies processes (T1/T1.5/T2)
- 📊 **Rate-Based Detection**: Catches fast & slow ransomware patterns
- 🔄 **Dynamic Budgets**: Per-tier WIP budgets
- 🛡️ **LockBit Resistant**: Defeats intermittent encryption attacks

---

## Architecture Overview

### V2.x Data Flow (Current)

```
Kernel (eBPF)              Userspace (Go)           Metrics
┌──────────────┐          ┌──────────────┐       ┌──────────┐
│sys_enter_    │          │              │       │          │
│  openat      │  Dwell   │    ADMM      │       │Prometheus│
│sys_enter_    ├─────────→│  Controller  ├──────→│Dashboard │
│  close       │  Event   │              │       │          │
│              │          │  Enforcement │       │          │
└──────────────┘          └──────────────┘       └──────────┘
```

**ADMM Update**: `price(t+1) = max(0, price(t) + α×(dwell(t) - budget))`

### V3.0 Architecture (Development)

```
Kernel (eBPF)              Userspace (Go)           Metrics
┌──────────────┐          ┌──────────────┐       ┌──────────┐
│kprobe/       │   WIP    │     TCM      │       │          │
│  vfs_write   │  Event   │  Classifier  │       │Prometheus│
│              ├─────────→│              ├──────→│Dashboard │
│TBW + UFM     │ (1s win) │    ADMM      │       │          │
│aggregation   │          │  (per tier)  │       │          │
└──────────────┘          └──────────────┘       └──────────┘
```

**TCM Tiers**:
| Tier | Profile | ω₁ | ω₂ | Budget |
|------|---------|----|----|--------|
| T1   | Backups | 0.9 | 0.1 | 12000 |
| T1.5 | Dev Builds | 0.55 | 0.45 | 8000 |
| T2   | Untrusted | 0.3 | 0.7 | 4000 |

---

## Documentation

| Audience | Resource |
|----------|----------|
| **End Users** | [User Guide](USER_GUIDE.md) — 5-minute setup, no jargon |
| **Developers** | [Architecture Docs](docs/architecture.md) — V2.x system design |
| **V3 Developers** | [V3 Quickstart](V3_QUICKSTART.md) — Integration guide |
| **Researchers** | [Stability Proofs](coq/dwell_stable.v) — Formal verification (⚠️ has errors) |
| **DevOps** | [Deployment Guide](docs/making-of.md) — systemd setup |
| **Release Notes** | [v1.3.0 Changelog](CHANGELOG.md) — Latest features |

### V3.0 Documentation
- [V3_MIGRATION_STATUS.md](V3_MIGRATION_STATUS.md) — **Complete V3 status & checklist**
- [V3_PIVOT_RESEARCH_DOSSIER.md](V3_PIVOT_RESEARCH_DOSSIER.md) — **Why V3? (LockBit problem)**
- [V3_QUICKSTART.md](V3_QUICKSTART.md) — Developer integration guide
- [ARCHITECTURE_V3.md](ARCHITECTURE_V3.md) — V2 vs V3 comparison

---

## Repository Structure

```
dwell-fiber/
├── bpf/                      # eBPF kernel programs (V2.x production)
│   ├── dwell_monitor.bpf.c   # File dwell time tracker
│   └── Makefile
├── coq/                      # Formal proofs (⚠️ has compilation errors)
│   ├── dwell_stable.v        # V2.x stability proof
│   └── Makefile
├── daemon/                   # Control daemon (Go - V2.x production)
│   ├── main.go              # Entry point
│   ├── controller.go        # ADMM implementation
│   └── metrics.go           # HTTP metrics server
├── pkg/                     # Reusable packages
│   ├── bpf/                 # BPF loader
│   └── enforcement/         # Throttle/kill logic
├── outputs/                 # V3.0 draft components (NOT integrated)
│   ├── dwell_monitor_v3.bpf.c     # V3 eBPF draft
│   ├── controller_v3.go           # V3 controller draft
│   └── V3_*.md                    # V3 documentation
├── USER_GUIDE.md           # End-user guide ⭐ START HERE
├── V3_MIGRATION_STATUS.md  # ⚠️ V3 development status
└── README.md               # This file
```

---

## Current Status (v1.3.0)

### V2.x Production Status ✅
- ✅ BPF monitoring active (sys_enter_openat/close)
- ✅ ADMM controller functional
- ✅ Enforcement live (throttle via cgroups v2, kill via signals)
- ✅ Metrics & dashboard working
- ✅ Safety checks (protected processes)
- ⚠️ Coq proofs have type unification errors (need fixing)
- ⚠️ Vulnerable to intermittent ransomware (LockBit pattern)

### V3.0 Development Status ⚠️
- ✅ Research complete (V3_PIVOT_RESEARCH_DOSSIER.md)
- ✅ Architecture designed
- ✅ Draft eBPF program created (outputs/dwell_monitor_v3.bpf.c)
- ✅ Draft controller created (outputs/controller_v3.go)
- ❌ Not integrated into build system
- ❌ Not compiled or tested
- ❌ Coq proofs not written
- ❌ E2E tests not created

**Estimated Completion**: 21-33 hours of development work

See [V3_MIGRATION_STATUS.md](V3_MIGRATION_STATUS.md) for detailed checklist.

---

## Performance (V2.x Measured)

Observed on Ubuntu 25.10 (kernel 6.17), Go 1.25:

- eBPF overhead: <100 μs per event
- Controller latency: <10ms per decision
- Memory usage: ~50-80 MB daemon
- CPU usage: <1% idle, spikes during enforcement
- Metrics update rate: 1Hz

---

## Security Considerations

⚠️ **This system requires root/CAP_BPF privileges**

**Why?**
- eBPF programs must be loaded into the kernel
- Enforcement requires killing/throttling processes
- Reading from kernel ring buffers requires privileges

**Best Practices:**
- Run daemon as systemd service with minimal privileges
- Use AppArmor/SELinux profiles to restrict daemon
- **Start in observation mode** (no `--enable-enforcement`)
- Monitor logs for anomalies

**Safe-by-Default**: Enforcement is OFF unless explicitly enabled with `--enable-enforcement` flag.

---

## Contributing to V3.0 Development

We welcome contributions to complete the V3.0 integration! Priority areas:

1. **Fix V2.x Coq Proofs** - Resolve type unification errors (blocker)
2. **Integrate V3 eBPF** - Compile and test dwell_monitor_v3.bpf.c
3. **Integrate V3 Controller** - Wire controller_v3.go into main.go
4. **Write V3 Coq Proofs** - Prove WIP convexity, tier-switching stability
5. **Testing** - Create V3 workload generators and E2E tests

**See [V3_QUICKSTART.md](V3_QUICKSTART.md) for integration guide.**

### Development Process
1. Create feature branch: `feature/v3-component-name`
2. Tag issues with `v3.0-migration`
3. Reference [V3_MIGRATION_STATUS.md](V3_MIGRATION_STATUS.md) checklist
4. Submit PR with tests

---

## License

MIT License - See [LICENSE](LICENSE) file

---

## Citation

If you use Dwell-Fiber in research, please cite:

```bibtex
@software{dwell_fiber_2025,
  title = {Dwell-Fiber: Formally-Verified Ransomware Defense},
  author = {dyb},
  year = {2025},
  version = {v1.3.0},
  url = {https://github.com/dyb5784/dwell-fiber},
  note = {V3.0 WIP-based architecture in development}
}
```

---

## Acknowledgments

Based on optimization-decomposition ideas for network architectures (Doyle & Chiang, 2007) integrated with formal verification techniques. Universal Decomposition Canon (UDC) Thoughtbase used to generate decomposition heuristics.

Key influences:
- Doyle & Chiang (2007) — "Layering as optimization decomposition"
- Dave Aitel (2016) — "Dwell Time" concept
- Daniel Miessler — Unsupervised Learning Newsletter

---

**Status**: V2.x Production-Ready | V3.0 Development In Progress  
**Latest Release**: v1.3.0 (November 6, 2025)  
**Last Updated**: November 15, 2025  
**Maintainer**: [@dyb5784](https://github.com/dyb5784)
