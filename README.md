# 🛡️ Dwell-Fiber

**Ransomware Defense Through Proven-Stable Economic Enforcement**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Ubuntu 25.10](https://img.shields.io/badge/Ubuntu-25.10-orange.svg)](https://ubuntu.com/)
[![Coq 8.18](https://img.shields.io/badge/Coq-8.18-blue.svg)](https://coq.inria.fr/)
[![Version: v1.8.0](https://img.shields.io/badge/Version-v1.8.0-green.svg)](https://github.com/chokmah-me/dwell-fiber/releases/tag/v1.8.0)
[![Build: Coq Verified](https://img.shields.io/badge/Build-Coq%20Verified-brightgreen.svg)](https://github.com/chokmah-me/dwell-fiber)

## Current status

v1.8.0 ships **V3 dual-mode** alongside production V2: rate-based WIP
observation (`--use-v3-wip`) detects the fast-intermittent-encryption pattern
V2 is blind to; opt-in enforcement (`--v3-enforce`, dry-run by default;
`--v3-enable-killing` separate) throttles/kills on that signal. V3 thresholds
are **calibrated and locked** into daemon defaults (V3ThrottlePrice=102.2,
V3KillPrice=204.4) from the third calibration pass (`P_b = 0.0`,
`P_i = 681.37`, all gates pass), with runtime override flags — and the armed
throttle test passed live on the WSL guest (7/7 gates: attack io.max-throttled
in 10 s, kill band provably disarmed, benign tar untouched). The opt-in **ACP
cognitive-phase price policy** (`--acp-policy`) modulates the V3 ADMM update
by inferred attacker phase (recon/learning/exploitation) — live-validated with
~2× attack-peak lift vs fixed pricing; see
[docs/acp-bridge.md](docs/acp-bridge.md). An observe-only **ambient-storm
distinguisher** (`daemon/ambient.go`) labels metronomic open-storm bursts
across PIDs and is wired into the calibration harness (`P_*_clean` peaks).
Event counters are counted **in-kernel before** the dwell filter. V3 process
names for tiering come from the BPF `wip_tracker` map
(`bpf_get_current_comm` at window create), not only `/proc`. Coq proofs are
**complete: 76/76 declarations, 0 admitted** (Coq 8.18.0), verified
fail-closed via `make verify`. See [STATUS.md](STATUS.md) and
[CHANGELOG.md](CHANGELOG.md).

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
| **ACP Bridge** | [ACP Cognitive-Phase Policy](docs/acp-bridge.md) |
| **Ambient Distinguisher** | [Ambient-vs-Enumeration Design](docs/ambient-distinguisher.md) |
| **V3 Calibration** | [V3 Threshold Calibration](docs/v3-calibration.md) |
| **Benchmarks** | [BENCHMARKS.md](BENCHMARKS.md) |
| **Project status** | [STATUS.md](STATUS.md) |
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
pattern: <100ms dwell per file across thousands of files). **V3 WIP** is
integrated in dual mode (`--use-v3-wip` / `--v3-enforce`); the third
calibration pass (2026-09-25, WSL) measured feasible separation
(`P_b = 0.0`, `P_i = 681.37` → throttle 102.2, kill 204.4, all gates pass)
after the BPF verifier fix and the map-stored `comm` tiering fix, and those
thresholds are **locked into daemon defaults** (2026-09-28) with the armed
throttle test passed live (7/7 gates, `test/wsl_throttle_test.sh`). Accepted
caveat: the worst ambient burst seen (~96 price) sits ~6% below the throttle
band and its source is not yet identified — the ambient-storm gauge now
labels such bursts live and enforcement stays observation-mode by default.
The **ACP cognitive-phase policy** (opt-in `--acp-policy`) modulates V3
pricing by attacker phase; live A/B (2026-09-25) showed ~2× attack-peak lift
vs fixed pricing (344.20 → 681.37) with gates passing either way. Original V3
drafts remain at tags `v3.0.0`–`v3.0.2` / `outputs/`. See
[docs/v3-roadmap.md](docs/v3-roadmap.md),
[docs/v3-calibration.md](docs/v3-calibration.md), [docs/acp-bridge.md](docs/acp-bridge.md),
[STATUS.md](STATUS.md).

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
@software{dwell_fiber_2026,
  title={Dwell-Fiber: Formally-Verified Ransomware Defense},
  author={Daniyel Yaacov Bilar},
  year={2026},
  version={v1.8.0},
  url={https://github.com/chokmah-me/dwell-fiber}
}
```

---

**Questions?** Open an [issue](https://github.com/chokmah-me/dwell-fiber/issues) or see [docs/](docs/)
