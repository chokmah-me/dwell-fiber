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
git clone https://github.com/chokmah-me/dwell-fiber.git
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

### Build Errors

**Error**: `permission denied` when starting daemon
- **Cause**: BPF loading requires root or CAP_BPF capability
- **Fix 1**: Run with `sudo ./bin/dwell-fiber-daemon`
- **Fix 2**: Grant capability: `sudo setcap cap_bpf=ep ./bin/dwell-fiber-daemon`

**Error**: `failed to load eBPF program`
- **Cause**: Kernel version too old
- **Fix**: Check kernel version (`uname -r` >= 5.8)

**Error**: `undefined reference to 'libbpf_*'`
- **Fix**: Install libbpf: `sudo apt-get install libbpf-dev`

**Error**: `package github.com/cilium/ebpf: no Go files`
- **Fix**: Download dependencies: `go mod download && go mod tidy`

**Error**: `coqc: not found`
- **Fix**: Install Coq: `sudo apt-get install coq && coq -v`

### Runtime Issues

**Problem**: Daemon starts but no BPF events appear
- **Solution 1**: Try simulation mode: `./bin/dwell-fiber-daemon --simulate`
- **Solution 2**: Check BPF loading: `dmesg | grep -i "dwell\|ebpf" | tail -10`
- **Solution 3**: Verify asm symlink: `ls -la /usr/include/asm`
- **Solution 4**: Rebuild: `make clean all`

**Problem**: No high-dwell events logged
- **Solution 1**: Generate workload: `cd test && go run workload_generator.go`
- **Solution 2**: Watch logs: `sudo tail -f /var/log/syslog | grep "High dwell"`
- **Note**: Events < 0.1s are filtered (background noise)
- **Note**: Events < 2s are skipped (normal operations)

**Problem**: Metrics endpoint not responding
- **Solution 1**: Check port: `netstat -tuln | grep 9090`
- **Solution 2**: Kill existing: `pkill dwell-fiber-daemon && sleep 1`
- **Solution 3**: Try different port: `./bin/dwell-fiber-daemon --port=9091`

**Problem**: Dashboard shows 0.0 for all values
- **Solution 1**: Run workload: `cd test && go run workload_generator.go`
- **Solution 2**: Wait 5 seconds then refresh (F5)
- **Solution 3**: Check metrics: `watch 'curl -s http://localhost:9090/metrics'`

### Enforcement Issues

**Problem**: Enforcement not triggering (no 🐌 or 💀 symbols)
- **Check 1**: Verify thresholds: `grep -n "ThrottleThreshold\|KillThreshold" daemon/controller.go`
  - Throttle: > 5 seconds
  - Kill: > 15 seconds
- **Check 2**: Generate critical workload: `cd test && go run workload_generator.go -critical=10`
- **Check 3**: Look for `[DRY-RUN]` in logs (safe testing mode)

**Problem**: Enforcement in dry-run but need real enforcement
- **Warning**: Real enforcement throttles/kills processes - use with caution!
- **Solution**: Add flags: `./bin/dwell-fiber-daemon --enable-enforcement --enable-killing`
- **Test first**: Always test with `--test-enforcement` before enabling real enforcement

### Coq Proof Issues

**Problem**: Coq compilation errors
- **Solution 1**: Check Coq version: `coq -v` (need 9.1+)
- **Solution 2**: Run from coq directory: `cd coq && make verify`
- **Solution 3**: Check for missing imports in error message

**Problem**: Admitted proofs shown
- **This is normal**: 60% of proofs complete (29/48), 40% admitted
- **See**: `docs/coq_status.md` for detailed proof breakdown

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
