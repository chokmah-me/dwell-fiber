# Installation Guide

Complete installation instructions for Dwell-Fiber v2.x on Ubuntu 25.10.

## Prerequisites

### System Requirements
- **OS**: Ubuntu 25.10 (tested), other Linux distributions may work
- **Kernel**: Linux 5.8+ (for eBPF CO-RE support)
- **RAM**: 512MB minimum, 1GB+ recommended
- **Disk**: 100MB for binaries + logs

### Required Packages

```bash
sudo apt-get update
sudo apt-get install -y \
  clang \
  llvm \
  libbpf-dev \
  golang-go \
  coq \
  make \
  git
```

### CRITICAL: Ubuntu 25.10 asm Symlink Fix

Ubuntu 25.10 has a known issue with eBPF header paths. Fix it before building:

```bash
sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm
```

**Why?** eBPF programs need `#include <asm/types.h>`, but Ubuntu 25.10 only provides `/usr/include/x86_64-linux-gnu/asm/`. This symlink makes the header discoverable.

---

## Installation

### Clone Repository

```bash
git clone https://github.com/dyb5784/dwell-fiber.git
cd dwell-fiber
```

### Build All Components

```bash
# Build daemon, eBPF programs, and dashboard
make all
```

**Build Artifacts**:
- `bin/dwell-fiber-daemon` - Main daemon (Go)
- `bpf/dwell_monitor.o` - eBPF bytecode
- `dashboard/` - Web UI (served by daemon)

### Verify Coq Proofs (Optional)

```bash
# Compile formal verification proofs
make verify
```

**Status**: 29/48 proofs complete (60%), 19 admitted. All files compile successfully.

---

## Quick Start

### 1. Observation Mode (Safe Default)

Start the daemon without enforcement to observe process behavior:

```bash
sudo ./bin/dwell-fiber-daemon --alpha=0.5 --budget=5.0
```

**Parameters**:
- `--alpha=0.5` - ADMM step size (0.1-2.0 range)
- `--budget=5.0` - Target dwell time in seconds

### 2. Check Status

In another terminal:

```bash
# Health check
curl http://localhost:9090/health

# View metrics
curl http://localhost:9090/metrics

# Open web UI
firefox http://localhost:9090
```

### 3. Enable Enforcement (Use with Caution)

**⚠️ WARNING**: This will throttle/kill processes exceeding the dwell budget!

```bash
sudo ./bin/dwell-fiber-daemon \
  --alpha=0.5 \
  --budget=5.0 \
  --enable-enforcement \
  --enable-killing
```

**Enforcement Thresholds** (hardcoded in v2.x):
- **Throttle**: 5 seconds dwell time
- **Kill**: 15 seconds dwell time

---

## Configuration

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--alpha` | 0.5 | ADMM step size (0.1-2.0) |
| `--budget` | 5.0 | Target dwell time (seconds) |
| `--enable-enforcement` | false | Enable process throttling |
| `--enable-killing` | false | Enable process killing |
| `--port` | 9090 | HTTP server port |
| `--log-level` | info | Log verbosity (debug/info/warn/error) |

### Example Configurations

**Conservative (recommended for testing)**:
```bash
sudo ./bin/dwell-fiber-daemon --alpha=0.3 --budget=10.0 --enable-enforcement
```

**Aggressive (ransomware defense)**:
```bash
sudo ./bin/dwell-fiber-daemon --alpha=1.0 --budget=5.0 --enable-enforcement --enable-killing
```

---

## Verification

### Test eBPF Program

```bash
# Check if eBPF program is loaded
sudo bpftool prog show | grep dwell

# View eBPF map contents
sudo bpftool map dump name process_prices
```

### Monitor Logs

```bash
# Real-time logs
sudo journalctl -u dwell-fiber -f

# Or check stdout if running in foreground
sudo ./bin/dwell-fiber-daemon --log-level=debug
```

---

## Troubleshooting

### Build Errors

**Error**: `fatal error: 'asm/types.h' file not found`
- **Fix**: Run the asm symlink fix (see Prerequisites)

**Error**: `undefined reference to 'libbpf_*'`
- **Fix**: `sudo apt-get install libbpf-dev`

### Runtime Errors

**Error**: `permission denied` when starting daemon
- **Fix**: Run with `sudo` (eBPF requires CAP_SYS_ADMIN)

**Error**: `failed to load eBPF program`
- **Fix**: Check kernel version (`uname -r` >= 5.8)

**Error**: No processes showing in dashboard
- **Fix**: Generate file activity (`cp /bin/ls /tmp/test-file`)

---

## Uninstallation

```bash
# Stop daemon (if running as service)
sudo systemctl stop dwell-fiber

# Remove binaries
make clean
cd .. && rm -rf dwell-fiber
```

---

## Next Steps

- [V2.x Architecture](v2-architecture.md) - Understand how it works
- [V3.0 Roadmap](v3-roadmap.md) - Upcoming features
- [Coq Proof Status](coq_status.md) - Formal verification details
- [CONTRIBUTING.md](../CONTRIBUTING.md) - Development guide
