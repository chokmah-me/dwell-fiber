# Making of Dwell-Fiber

## Project Genesis

I started Dwell-Fiber as a practical demonstration of the **fibered-functor compiler framework**, showing how I could apply Network-Utility-Maximisation (NUM) principles to system resource management. The project shows how compiler passes can be viewed as contravariant functors over resource budgets, with strong theoretical guarantees.

## Core Design: The 300-Line Heart

The system's core is a compact 300-line implementation that demonstrates the "same math, different fibre" principle. Through **sigil remapping**, this core has been successfully adapted to various resource management scenarios:

1. Ransomware dwell-time control (my repo's primary focus)
2. ATP-budgeted metabolic edits in biological systems
3. Cache-constrained thread placement in HPC environments

## Implementation Choices

### eBPF + Go Architecture
- eBPF for kernel-level monitoring with minimal overhead
- Go daemon for robust userspace coordination
- ADMM-based price updates for proven convergence
- K8s integration for cloud-native deployment

### Formal Verification
- Coq proofs ensure system stability
- Chordal dependence graphs guarantee bounded Lyapunov drift
- Full mechanized verification in under 180ms

## Toolchain Requirements

### Core Build Tools
- Go 1.21 or later
- LLVM/Clang 16.0+
- Linux kernel headers (≥ 5.15)
- Coq 8.16 or later
- GNU Make

### Optional Tools
- kubectl for K8s deployment
- perf for performance analysis
- graphviz for dependency visualization

## Reproduction Guide

### Basic Build
```bash
# Clone the repo
git clone https://github.com/chokmah-me/dwell-fiber.git
cd dwell-fiber

# Fix asm/types.h symlink (critical first step!)
chmod +x scripts/fix-asm-symlink.sh
sudo ./scripts/fix-asm-symlink.sh

# Build all components
make all

# Verify proofs (fast)
make verify
```

### Running Tests
```bash
# Core tests
make test

# Performance benchmarks
make bench

# Full CI suite
make ci
```

### Performance Measurement
```bash
# Measure syscall overhead
perf stat -r 10 ./bin/dwell-fiber-daemon --bench-syscall

# Time Coq verification
make time-verify
```

## Known Issues & Workarounds

1. **asm/types.h symlink**: Must be fixed before building (see fix-asm-symlink.sh)
2. **Go version compatibility**: Tested primarily with Go 1.21
3. **K8s metric registration**: May need cluster-admin role

See [troubleshooting.md](troubleshooting.md) for more details.

## Future Directions

1. **Extended Fibrations**: Support for more resource types
2. **Runtime Verification**: Online proof checking
3. **Cloud Integration**: Native cloud provider metrics

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines and best practices.

## UDC essentials (summary)

I distilled a set of domain-neutral heuristics (the Universal Decomposition Canon) that guided decomposition choices during design. I do not publish the internal Thoughtbase here; instead the essential recipe I used is:

- Identify the decision variables that are being tuned.
- Identify the scarce resources and coupling constraints that bind the variables.
- Make the implicit objective/utility explicit (and where possible, convexify it).
- Choose a decomposition strategy (dual/primal/ADMM/penalty/consensus) guided by communication cost and observability.
- Produce a simple stability certificate (Lyapunov, contraction or passivity style) for the chosen iterative updates.

This short recipe is intentionally high-level — it captures the engineering moves without exposing the private Thoughtbase representations.