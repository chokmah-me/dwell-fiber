# 🛡️ Dwell-Fiber

**Ransomware Defense Through Proven-Stable Economic Enforcement**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Ubuntu 25.10](https://img.shields.io/badge/Ubuntu-25.10-orange.svg)](https://ubuntu.com/)
[![Coq 9.1+](https://img.shields.io/badge/Coq-9.1%2B-blue.svg)](https://coq.inria.fr/)
[![Version: v1.5.0](https://img.shields.io/badge/Version-v1.5.0-green.svg)](https://github.com/dyb5784/dwell-fiber/releases/tag/v1.5.0)
[![Build: Coq Verified](https://img.shields.io/badge/Build-Coq%20Verified-brightgreen.svg)](https://github.com/dyb5784/dwell-fiber)

## Current status

v1.5.0 ships a correctness fix for concurrent file-descriptor tracking
and a benchmark harness with empirical results. See [STATUS.md](STATUS.md)
for what's working, what's frozen, and what isn't happening.

---

## What is Dwell-Fiber?

Dwell-Fiber prevents ransomware by monitoring file access patterns and applying economic penalties via **ADMM optimization** (Alternating Direction Method of Multipliers). It uses eBPF for kernel-level tracking with minimal overhead.

**V2.x (Production)**: Tracks how long processes hold files open ("dwell time"). Throttles/kills processes exceeding a 5-second budget.

**V3.0 (Development)**: Rate-based detection using bytes written + files modified to catch fast intermittent ransomware attacks (LockBit 3.0+).

---

## Quick Start

### Installation

```bash
git clone https://github.com/chokmah-me/dwell-fiber.git
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

## Scope

**V2.x (current):** real-time eBPF dwell tracking, ADMM economic enforcement
(throttle via cgroups v2; kill via SIGTERM/SIGKILL), Prometheus metrics, web
dashboard. Catches sustained-dwell attack patterns. See `BENCHMARKS.md` for
measured behavior.

**Known gap:** V2 cannot catch fast intermittent encryption (LockBit 3.0
pattern: <100ms dwell per file across thousands of files). The V3.0 WIP-based
architecture explores a fix and is research-in-progress on the
`feature/v3-wip-architecture` branch. See [docs/v3-roadmap.md](docs/v3-roadmap.md).
No timeline.

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
└── test/                 # Integration and unit tests
```

### Testing

Run unit tests locally:
```bash
make test  # cd daemon && go test -v ./...
```

Scheduled tests run weekly via GitHub Actions. See `.github/workflows/` for CI configuration.

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
  title={Dwell-Fiber: Formally-Verified Ransomware Defense},
  author={Daniyel Yaacov Bilar},
  year={2025},
  version={v1.5.0},
  url={https://github.com/chokmah-me/dwell-fiber}
}
```

---

**Questions?** Open an [issue](https://github.com/dyb5784/dwell-fiber/issues) or see [docs/](docs/)
