# V3.0 Roadmap - WIP-Based Architecture

**Status**: 🚧 **Experimental** - On feature branch `feature/v3-wip-architecture`

V3.0 replaces dwell-time tracking with **rate-based I/O pressure** detection to catch fast intermittent ransomware attacks.

---

## The V2.x Problem

### What V2.x Cannot Detect

**Attack Pattern**: LockBit 3.0+ Fast Intermittent Encryption

```
for each file in /documents:
    open(file)
    encrypt_chunk(1MB)    # <100ms
    close(file)           # Dwell time: 100ms
    # Repeat 1000s of times
```

**Why V2.x Fails**:
- Each file session: <100ms dwell (well below 5s budget)
- ADMM price stays near 0
- No enforcement triggered
- All files encrypted in minutes

**Real-World Impact**: LockBit 3.0 encrypts 100GB in ~7 minutes with zero detection by V2.x

---

## V3.0 Solution: WIP (Weighted I/O Pressure)

### Core Concept

Instead of tracking **latency** (dwell time), track **rate**:
- **TBW** (Total Bytes Written): MB/s over 1-second windows
- **UFM** (Unique Files Modified): Files/s over 1-second windows
- **WIP = ω₁·TBW + ω₂·UFM**: Weighted combination

**Key Insight**: Ransomware has **high TBW + high UFM** regardless of session duration.

---

## Architecture

### Data Flow

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

### Components

#### 1. eBPF WIP Tracker

**Hooks**:
- `kprobe/vfs_write` - Track bytes written
- `kprobe/vfs_open` - Track unique files modified

**Per-Process Metrics** (1-second windows):
```c
struct wip_metrics {
    u32 pid;
    u64 total_bytes_written;  // Bytes in last 1s
    u32 unique_files_modified; // Files touched in last 1s
    u64 timestamp;            // Window start
};
```

**Aggregation**:
- Hash map: `PID → wip_metrics`
- Timer: Every 1s, publish metrics to userspace, reset counters

#### 2. TCM (Tier Classification Module)

**What it does**:
- Classifies processes into tiers based on behavioral profile
- Assigns tier-specific weights (ω₁, ω₂) and budgets

**Tiers**:

| Tier | Profile | ω₁ (TBW) | ω₂ (UFM) | Budget | Example Processes |
|------|---------|----------|----------|--------|-------------------|
| **T1** | High-throughput legitimate | 0.9 | 0.1 | 12000 | `rsync`, `tar`, `dd` |
| **T1.5** | Development workloads | 0.55 | 0.45 | 8000 | `gcc`, `npm`, `docker build` |
| **T2** | Untrusted / Unknown | 0.3 | 0.7 | 4000 | Unknown binaries |

**Classification Logic**:
```go
if process_name in ["rsync", "tar", "backup"] {
    tier = T1
} else if process_name in ["gcc", "cargo", "npm"] {
    tier = T1.5
} else {
    tier = T2  // Default: strict
}
```

**Future**: ML-based classification using process behavior history

#### 3. Per-Tier ADMM

**WIP Calculation**:
```
WIP = ω₁ × (TBW / 1MB) + ω₂ × (UFM / 1 file)
```

**ADMM Update**:
```
price(t+1) = max(0, price(t) + α × (WIP(t) - budget))
```

**Enforcement**:
```
if price >= kill_threshold:
    kill_process(pid)
elif price >= throttle_threshold:
    throttle_io(pid, 10%)  # cgroups v2 io.max
```

---

## Detection Examples

### Scenario 1: LockBit 3.0 (Fast Intermittent)

**Behavior**:
- TBW: 800 MB/s (encrypting 1MB chunks × 800 files/s)
- UFM: 800 files/s

**V3.0 Response**:
- Tier: T2 (unknown binary)
- ω₁=0.3, ω₂=0.7, budget=4000
- WIP = 0.3 × 800 + 0.7 × 800 = **800**
- Price update: `price += 0.5 × (800 - 4000)` → increases rapidly
- **Enforcement**: Throttle after 1-2 seconds, kill after 5 seconds

**Result**: ✅ **Detected and stopped**

### Scenario 2: Backup (rsync)

**Behavior**:
- TBW: 600 MB/s (copying large files)
- UFM: 50 files/s

**V3.0 Response**:
- Tier: T1 (`rsync` whitelisted)
- ω₁=0.9, ω₂=0.1, budget=12000
- WIP = 0.9 × 600 + 0.1 × 50 = **545**
- Price stays low (545 << 12000)
- **Enforcement**: None

**Result**: ✅ **No false positive**

### Scenario 3: Dev Build (npm install)

**Behavior**:
- TBW: 200 MB/s (extracting node_modules)
- UFM: 5000 files/s (many small files)

**V3.0 Response**:
- Tier: T1.5 (`npm` whitelisted)
- ω₁=0.55, ω₂=0.45, budget=8000
- WIP = 0.55 × 200 + 0.45 × 5000 = **2360**
- Price stays moderate (2360 << 8000)
- **Enforcement**: None

**Result**: ✅ **No false positive**

---

## Implementation Status

### ✅ Completed (on feature branch)

- Research & empirical analysis (see `V3_PIVOT_RESEARCH_DOSSIER.md` on branch)
- TCM tier design & weight tuning
- WIP formula validation via simulation

### 🚧 In Progress

- eBPF `vfs_write` kprobe implementation
- Unique file tracking (hash map: PID → set of inodes)
- 1-second windowing with timer

### ❌ Not Started

- Per-tier ADMM controller in userspace
- I/O throttling via cgroups v2 `io.max`
- ML-based tier classification
- Integration testing with real ransomware samples

---

## Migration Path

### Phase 1: Dual Mode (v1.5.0 - Planned)

- Run V2.x (dwell) + V3.0 (WIP) **in parallel**
- V2.x: Enforcement enabled (production)
- V3.0: Observation only (metrics collection)
- Compare false positive rates

### Phase 2: V3.0 Opt-In (v1.6.0 - Planned)

- Flag: `--use-v3-wip` to enable V3.0 enforcement
- Default: V2.x (stable)
- Collect production telemetry

### Phase 3: V3.0 Default (v2.0.0 - Future)

- V3.0 becomes default
- V2.x available via `--legacy-dwell-mode`
- Deprecate V2.x in v2.1.0

---

## Configuration (Planned)

### Tier Weights (v3-config.yaml)

```yaml
tiers:
  T1:
    name: "high-throughput"
    omega_tbw: 0.9
    omega_ufm: 0.1
    budget: 12000
    processes:
      - rsync
      - tar
      - backup-script
  T1.5:
    name: "development"
    omega_tbw: 0.55
    omega_ufm: 0.45
    budget: 8000
    processes:
      - gcc
      - cargo
      - npm
      - docker
  T2:
    name: "untrusted"
    omega_tbw: 0.3
    omega_ufm: 0.7
    budget: 4000
    processes: []  # Default tier
```

### Command-Line Flags

```bash
sudo ./bin/dwell-fiber-daemon \
  --use-v3-wip \                  # Enable V3.0 mode
  --v3-config=v3-config.yaml \    # Tier configuration
  --enable-enforcement \
  --enable-killing
```

---

## Performance Expectations

### Overhead (Estimated)

| Component | Latency Impact | CPU Usage |
|-----------|----------------|-----------|
| `vfs_write` kprobe | +200 ns/write | 0.5% |
| Unique file tracking | +100 ns/open | 0.2% |
| 1s aggregation | Negligible | 0.1% |
| **Total** | **<1%** | **0.8%** |

**Comparison to V2.x**: Similar overhead, slightly higher due to per-write tracking (vs V2.x open/close only)

### Memory (Estimated)

- Per-process WIP metrics: 64 bytes
- Unique file set (hash map): ~4KB per 100 files
- **Total for 1000 processes**: ~20 MB

---

## Risks & Unknowns

### 🔴 High Risk

1. **False Positives on Video Encoding**
   - ffmpeg writing 4K video: 100 MB/s, 1 file/s
   - WIP (T2): 0.3 × 100 + 0.7 × 1 = 30.7 → May trigger if sustained
   - **Mitigation**: Whitelist common media apps in T1

2. **I/O Throttling Stability**
   - cgroups v2 `io.max` can cause application hangs if misconfigured
   - **Mitigation**: Throttle to 10% (not 0%), gradual ramp-down

### 🟡 Medium Risk

3. **Timer Overhead**
   - 1-second timer may introduce jitter on high-load systems
   - **Mitigation**: Use high-resolution timers (`hrtimer` in eBPF)

4. **eBPF Map Size Limits**
   - Unique file tracking: 1 million inodes × 64 bytes = 64 MB
   - **Mitigation**: LRU eviction for old entries

### 🟢 Low Risk

5. **Tier Misclassification**
   - Unknown legitimate app classified as T2
   - **Mitigation**: User-configurable whitelist

---

## Research References

- **V3_PIVOT_RESEARCH_DOSSIER.md** (on feature branch) - Empirical WIP analysis
- **LockBit 3.0 Analysis** - Fast intermittent encryption patterns
- **ADMM for Networked Systems** (Boyd et al., 2010) - Theoretical foundation

---

## Get Involved

**Try V3.0 (Experimental)**:
```bash
git checkout feature/v3-wip-architecture
cd dwell-fiber
make all
sudo ./bin/dwell-fiber-daemon --use-v3-wip
```

**Contribute**:
- See [TODO.md](../TODO.md) - V3.0 tasks
- Join discussion: GitHub Issues tagged `v3-wip`

---

## Next Steps

- **Understand V2.x**: [V2 Architecture](v2-architecture.md)
- **Install**: [Installation Guide](installation.md)
- **Contribute**: [CONTRIBUTING.md](../CONTRIBUTING.md)
