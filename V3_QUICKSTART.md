# Dwell-Fiber V3.0 Developer Quickstart

## Current State Assessment

**Repository Status**: V2.x production code + V3.0 documentation/drafts

```
✅ What Works (V2.x):
- eBPF dwell-time monitoring
- ADMM price updates
- Enforcement (throttle/kill)
- Metrics & dashboard

❌ What's Missing (V3.0):
- WIP metric eBPF program (exists as draft)
- TCM tier classification (exists as draft)
- Integrated V3 controller (exists as draft)
- V3.0 Coq proofs (not started)
```

---

## Quick Integration Path (3-4 Hours)

### Step 1: Create Feature Branch (5 min)

```bash
cd ~/dwell-fiber
git checkout -b feature/v3-wip-integration
git push -u origin feature/v3-wip-integration
```

---

### Step 2: Integrate V3 Controller (1 hour)

#### 2.1 Copy Draft Controller
```bash
# Assuming outputs/ directory has the draft
cp /mnt/user-data/outputs/controller_v3.go daemon/controller_v3.go
```

#### 2.2 Add Conditional V3 Mode to main.go

```go
// daemon/main.go
import "flag"

func main() {
    // Existing flags
    alpha := flag.Float64("alpha", 0.5, "ADMM step size")
    budget := flag.Float64("budget", 5.0, "Target dwell time budget (seconds)")
    
    // NEW: V3 mode flag
    v3Mode := flag.Bool("v3", false, "Use V3.0 WIP-based controller (experimental)")
    
    flag.Parse()
    
    // Create appropriate controller
    if *v3Mode {
        log.Println("⚠️  Running in V3.0 mode (experimental WIP metric)")
        controllerV3 := NewControllerV3(*alpha)
        
        // TODO: Connect to V3 BPF monitor
        log.Fatal("V3 BPF integration not yet complete")
    } else {
        log.Println("Running in V2.x mode (dwell-time metric)")
        controller := NewController(*alpha, *budget)
        
        // Existing V2 logic...
    }
}
```

#### 2.3 Test Compilation
```bash
cd daemon
go mod tidy
go build -o ../bin/dwell-fiber-daemon-v3-test .
```

**Expected**: Compilation succeeds (V3 not yet wired to BPF)

---

### Step 3: Compile V3 eBPF Program (1 hour)

#### 3.1 Copy Draft eBPF Program
```bash
cp /mnt/user-data/outputs/dwell_monitor_v3.bpf.c bpf/dwell_monitor_v3.bpf.c
```

#### 3.2 Update Makefile
```makefile
# bpf/Makefile
TARGETS := dwell_monitor.bpf.o dwell_monitor_v3.bpf.o  # Add V3 target

dwell_monitor_v3.bpf.o: dwell_monitor_v3.bpf.c
	$(CLANG) $(BPF_CFLAGS) $(BPF_INCLUDES) -c $< -o $@
	@echo "✓ Compiled V3: $@"
```

#### 3.3 Attempt Compilation
```bash
cd bpf
make dwell_monitor_v3.bpf.o
```

**Expected Issues**:
- `struct file` access needs kernel headers
- `PT_REGS_PARM` macros may need adjustment
- Inode extraction requires BPF helpers

#### 3.4 Fix Compilation (Simplified Version)

```c
// Simplified version that compiles but needs refinement
SEC("kprobe/vfs_write")
int track_vfs_write(struct pt_regs *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 now = bpf_ktime_get_ns();
    
    // Simplified: Just track PID and timestamp for now
    // Full inode/TBW tracking requires more kernel structure access
    
    struct wip_state *state = bpf_map_lookup_elem(&wip_tracker, &pid);
    // ... rest of logic
    
    return 0;
}
```

---

### Step 4: Update BPF Loader (30 min)

```go
// pkg/bpf/loader.go

// Add V3 event struct
type WIPEvent struct {
    PID         uint32
    TBW         uint64
    UFM         uint64
    TimestampNs uint64
    Comm        [16]byte
}

// Add V3 loader function
func LoadBPFV3(objPath string) (*BPFManager, error) {
    // Similar to LoadBPF but expects WIPEvent
    // ...
}
```

---

### Step 5: Wire Together (1 hour)

```go
// daemon/main.go - V3 mode branch

if *v3Mode {
    controllerV3 := NewControllerV3(*alpha)
    
    // Load V3 BPF program
    bpfManager, err := bpf.LoadBPFV3("bpf/dwell_monitor_v3.bpf.o")
    if err != nil {
        log.Fatalf("Failed to load V3 BPF: %v", err)
    }
    defer bpfManager.Close()
    
    // Start event processor
    go func() {
        for event := range bpfManager.Events {
            // Cast to WIPEvent
            wipEvent := event.(bpf.WIPEvent)
            
            // Extract metrics
            tbw := float64(wipEvent.TBW)
            ufm := float64(wipEvent.UFM)
            cmd := bpf.GetString(wipEvent.Comm[:])
            
            // Process via V3 controller
            controllerV3.HandleWIPEvent(int(wipEvent.PID), cmd, tbw, ufm)
        }
    }()
    
    // Start metrics server (same as V2)
    go StartMetricsServer(*port, controllerV3)
}
```

---

### Step 6: Test V3 Mode (30 min)

```bash
# Terminal 1: Start V3 daemon
sudo ./bin/dwell-fiber-daemon --v3 --alpha=0.5

# Terminal 2: Generate I/O workload
dd if=/dev/zero of=/tmp/test1.bin bs=1M count=100
dd if=/dev/zero of=/tmp/test2.bin bs=1M count=100
# ... create multiple files quickly

# Terminal 3: Monitor metrics
curl http://localhost:9090/metrics | grep wip

# Expected metrics (V3):
# dwell_fiber_wip_current 5000
# dwell_fiber_tier 2
# dwell_fiber_price 0.25
```

---

## Validation Checklist

### Compilation
- [ ] `make bpf` includes `dwell_monitor_v3.bpf.o`
- [ ] V3 .o file exists: `ls -lh bpf/dwell_monitor_v3.bpf.o`
- [ ] `go build` succeeds with controller_v3.go

### Runtime (V3 Mode)
- [ ] Daemon starts with `--v3` flag
- [ ] BPF program loads: `sudo bpftool prog list | grep vfs_write`
- [ ] Events received: Check logs for WIPEvent processing
- [ ] Tier classification works: See tier switches in logs
- [ ] Metrics exported: `curl http://localhost:9090/metrics`

### Functionality
- [ ] High TBW workload → Classified as T1
- [ ] High UFM workload → Classified as T2
- [ ] WIP calculated correctly
- [ ] Price updates based on WIP violation
- [ ] Dashboard shows WIP instead of dwell time

---

## Known Limitations (V3.0 Alpha)

### eBPF Simplifications
- Inode tracking may be incomplete (needs proper struct file access)
- TBW counting relies on write size from context
- 256-slot inode bitmap has collisions for > 256 unique files

### Controller
- Tier switching not yet optimized
- No tier-specific enforcement policies yet
- Enforcement still uses dwell-time thresholds (needs WIP thresholds)

### Formal Verification
- V3.0 Coq proofs not written
- V2.x proofs have compilation errors (blocker)

---

## Immediate Next Steps

### For Testing (Non-Root)
1. Create V3 simulation mode (no BPF)
   ```go
   // daemon/main.go
   if *v3Mode && *simulate {
       // Generate synthetic WIP events
       go runV3SimulationLoop(controllerV3)
   }
   ```

2. Add V3 test scenarios
   ```go
   // daemon/test_suite_v3.go
   func GenerateV3TestScenarios() []WIPScenario {
       return []WIPScenario{
           {Name: "Backup", TBW: 10000, UFM: 100, Tier: T1},
           {Name: "Build", TBW: 1000, UFM: 50000, Tier: T1_5},
           {Name: "Ransomware", TBW: 500, UFM: 100000, Tier: T2},
       }
   }
   ```

### For Production (Root Required)
1. Fix eBPF inode extraction
2. Test with real workloads
3. Validate tier classification accuracy
4. Benchmark performance overhead

---

## Full V3.0 Completion Estimate

| Task | Time | Difficulty |
|------|------|------------|
| Fix eBPF compilation | 2-4h | Medium |
| Integrate controller | 1-2h | Low |
| Update BPF loader | 1-2h | Low |
| Wire everything | 2-3h | Medium |
| Testing & debugging | 4-6h | Medium |
| **Basic V3 Working** | **10-17h** | - |
| Add Coq proofs | 6-10h | High |
| Comprehensive testing | 4-6h | Medium |
| Documentation | 2-3h | Low |
| **Production V3.0** | **22-36h** | - |

---

## Getting Help

### Issues to File
- Tag with `v3.0-migration`
- Include error logs
- Note which step you're on

### Reference Documents
- [V3_PIVOT_RESEARCH_DOSSIER.md](V3_PIVOT_RESEARCH_DOSSIER.md) - Why V3?
- [V3_MIGRATION_STATUS.md](V3_MIGRATION_STATUS.md) - Detailed status
- [ARCHITECTURE_V3.md](ARCHITECTURE_V3.md) - Architecture comparison

### Debugging
```bash
# Check BPF loading
sudo bpftool prog list
sudo dmesg | grep -i bpf

# Check V3 events
sudo bpftool map dump name wip_tracker

# Monitor metrics
watch -n1 'curl -s http://localhost:9090/metrics | grep dwell_fiber'
```

---

**Status**: V3.0 development quickstart  
**Target**: Basic V3 working in 10-17 hours  
**Last Updated**: 2025-11-15