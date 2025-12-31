# 🛡️ Dwell-Fiber

**Ransomware Defense Through Proven-Stable Economic Enforcement**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Ubuntu 25.10](https://img.shields.io/badge/Ubuntu-25.10-orange.svg)](https://ubuntu.com/)
[![Coq 9.1+](https://img.shields.io/badge/Coq-9.1%2B-blue.svg)](https://coq.inria.fr/)
[![Version: v1.4.2](https://img.shields.io/badge/Version-v1.4.2-green.svg)](https://github.com/dyb5784/dwell-fiber/releases/tag/v1.4.2)
[![Build: Coq Verified](https://img.shields.io/badge/Build-Coq%20Verified-brightgreen.svg)](https://github.com/dyb5784/dwell-fiber)

## ✅ Current Status: v1.4.2 - Coq Proof Compilation Fixed

**Latest Release**: v1.4.2 (December 30, 2025)

All Coq proofs compile successfully with `make verify` - 29/48 proofs complete (60%), full verification build working.

---

## What is Dwell-Fiber?

Dwell-Fiber prevents ransomware by monitoring file access patterns and applying economic penalties via **ADMM optimization** (Alternating Direction Method of Multipliers). It uses eBPF for kernel-level tracking with minimal overhead.

**V2.x (Production)**: Tracks how long processes hold files open ("dwell time"). Throttles/kills processes exceeding a 5-second budget.

**V3.0 (Development)**: Rate-based detection using bytes written + files modified to catch fast intermittent ransomware attacks (LockBit 3.0+).

---

## Quick Start

### Installation

```bash
git clone https://github.com/dyb5784/dwell-fiber.git
cd dwell-fiber
make all
```

**Full setup guide**: [Installation Guide](docs/installation.md)

### Run (Observation Mode)

```bash
sudo ./bin/dwell-fiber-daemon --alpha=0.5 --budget=5.0
```

Visit `http://localhost:9090` for the dashboard.

**Enable enforcement** (use with caution):
```bash
sudo ./bin/dwell-fiber-daemon --enable-enforcement --enable-killing
```

---

## Documentation

| Topic | Link |
|-------|------|
| **Installation** | [Installation Guide](docs/installation.md) |
| **V2.x Architecture** | [V2 Architecture](docs/v2-architecture.md) |
| **V3.0 Roadmap** | [V3 Roadmap](docs/v3-roadmap.md) |
| **Coq Proofs** | [Coq Status](docs/coq_status.md) |
| **Contributing** | [CONTRIBUTING.md](CONTRIBUTING.md) |
| **Changelog** | [CHANGELOG.md](CHANGELOG.md) |

---

## Features

### V2.x (Production)
- ✅ Real-time eBPF-based file dwell tracking
- ✅ ADMM economic enforcement (throttle + kill)
- ✅ Prometheus metrics + web dashboard
- ✅ Formal verification framework (60% proven)
- ✅ Sub-millisecond latency impact

### V3.0 (Planned)
- 🚧 Rate-based detection (TBW + UFM metrics)
- 🚧 Tier classification (T1/T1.5/T2 budgets)
- 🚧 Defeats LockBit 3.0 intermittent encryption

---

## How It Works

**ADMM Price Update**:
```
price(t+1) = max(0, price(t) + α × (dwell(t) - budget))
```

- **Normal processes**: Dwell time < budget → price stays at 0
- **Ransomware**: Dwell time >> budget → price increases rapidly → throttle/kill

**Example** (α=0.5, budget=5s):
- File held for 10s → `price += 0.5 × (10 - 5) = 2.5`
- After 3 files @ 10s each → price ≈ 7.5 → **throttled**
- After 6 files → price ≈ 15 → **killed**

See [V2 Architecture](docs/v2-architecture.md) for full details.

---

## Repository Structure

```
dwell-fiber/
├── bpf/                  # eBPF kernel programs
├── daemon/               # Go userspace daemon
├── coq/                  # Formal verification (Coq proofs)
├── dashboard/            # Web UI
├── docs/                 # Documentation
└── tests/                # Integration tests
```

---

## Performance

- **Latency**: +100ns per file operation
- **CPU**: <1% (observation), <3% (enforcement)
- **Memory**: 12-18 MB

See [V2 Architecture - Performance](docs/v2-architecture.md#performance-measured) for benchmarks.

---

## Security Note

⚠️ **Known Limitation**: V2.x cannot detect fast intermittent encryption (LockBit 3.0+). See [V3 Roadmap](docs/v3-roadmap.md) for solution.

**Not a replacement** for antivirus/EDR. Use as defense-in-depth layer.

---

## Acknowledgments

Based on optimization-decomposition ideas for network architectures integrated with formal verification techniques.

**Key Influences**:
- **Doyle & Chiang (2007)** — "Layering as optimization decomposition: A mathematical theory of network architectures"
- **Dave Aitel (2016)** — "Dwell Time" concept for intrusion detection
- **Daniel Miessler** — Unsupervised Learning Newsletter (security insights)

**Techniques**:
- ADMM optimization (Boyd et al., 2010)
- eBPF CO-RE (Compile Once, Run Everywhere)
- Coq formal verification framework

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Development setup
- Code style guidelines
- Testing requirements
- Coq proof development

---

## License

MIT License - See [LICENSE](LICENSE)

---

## Citation

```bibtex
@software{dwell_fiber_2025,
  title={Dwell-Fiber: ADMM-Based Ransomware Defense},
  author={Your Name},
  year={2025},
  url={https://github.com/dyb5784/dwell-fiber}
}
```

---

**Questions?** Open an [issue](https://github.com/dyb5784/dwell-fiber/issues) or see [docs/](docs/)
