#!/bin/bash
# Quick reference for running tests
# Run this to see all available test commands

cat << 'EOF'
╔════════════════════════════════════════════════════════════════════╗
║           DWELL-FIBER TEST SUITE - QUICK REFERENCE                ║
╚════════════════════════════════════════════════════════════════════╝

█ SETUP (One-time)
  Ubuntu 25.10 critical fix:
  $ sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm

█ BUILD
  $ make clean all        # Build everything (BPF + Coq + Go)
  $ make daemon           # Build just daemon
  $ make bpf              # Build just eBPF program
  $ make verify           # Verify Coq proofs

█ SIMULATION MODE (No root required)
  Terminal 1 - Start daemon:
  $ ./bin/dwell-fiber-daemon --simulate
  
  Terminal 2 - Open dashboard:
  $ firefox http://localhost:9090
  
  Terminal 3 - Monitor metrics:
  $ watch 'curl -s http://localhost:9090/metrics'

█ REAL BPF MODE (Requires root + CAP_BPF)
  Terminal 1 - Start daemon:
  $ sudo ./bin/dwell-fiber-daemon --alpha=0.5 --budget=5.0
  
  Terminal 2 - Open dashboard:
  $ firefox http://localhost:9090
  
  Terminal 3 - Generate high-dwell workload:
  $ cd test && go run workload_generator.go
  
  Terminal 4 - Monitor enforcement:
  $ grep "High dwell\|Throttle\|Kill" /var/log/syslog

█ WORKLOAD GENERATION
  Generate all workload types (5, 7, 9 second dwell):
  $ cd test && go run workload_generator.go
  
  Or from Go code:
  $ go run test/workload_generator.go -normal=2 -high=3 -critical=2

█ FULL INTEGRATION TEST
  Automated end-to-end test with workload + metrics + validation:
  $ chmod +x test/test-suite.sh
  $ ./test/test-suite.sh
  
  Monitor output and check Firefox dashboard at http://localhost:9090

█ DEBUG COMMANDS
  Check BPF loaded:
  $ dmesg | grep -i "dwell\|ebpf" | tail -10
  
  Check metrics endpoint:
  $ curl http://localhost:9090/metrics
  
  Check daemon logs:
  $ tail -50 /tmp/dwell-fiber-test/daemon.log
  
  Verify symlink fix:
  $ ls -la /usr/include/asm
  
  Check enforcement capability:
  $ getcap ./bin/dwell-fiber-daemon
  
  Monitor system logs (real BPF mode):
  $ tail -f /var/log/syslog | grep -i "dwell\|enforce"

█ EXPECTED OUTPUTS

  ✓ Simulation Mode:
    - Generates synthetic dwell patterns
    - Price updates follow ADMM formula
    - Dashboard shows price convergence
    - No enforcement (simulation only)
  
  ✓ Real BPF Mode (with workload):
    - BPF events captured: "✓ BPF program loaded"
    - High dwell logged: "⏱️  High dwell: PID=12345 (bash) dwell=7.00s"
    - Enforcement decisions: "🐌 [DRY-RUN] Would throttle PID=12345"
    - Price increases: "Price: 0.100 → 0.350 → 0.500+"
    - Dashboard updates in real-time
  
  ✓ Workload Generator:
    - Creates files in /tmp/dwell-fiber-workload/
    - Keeps files open for specified durations
    - Logs: "✓ Idle operation (0.5s)"
    -       "✓ Normal operation (5s)"
    -       "✓ High operation (7s, throttle threshold)"
    -       "✓ Critical operation (9s, kill threshold)"

█ TESTING MATRIX

  Simulation Mode (no BPF):
    ✓ ADMM algorithm convergence
    ✓ Price updates
    ✓ Metrics generation
    ✓ Web UI responsiveness
  
  Real BPF Mode:
    ✓ eBPF program loading
    ✓ Ring buffer event capture
    ✓ Noise filtering (< 0.1s dropped)
    ✓ Controller filtering (< 2s skipped)
    ✓ High-dwell event logging
    ✓ Enforcement decisions (throttle/kill)
    ✓ Prometheus metrics
    ✓ Web dashboard
  
  Integration Test:
    ✓ Build succeeds
    ✓ Daemon starts
    ✓ Workload generates high dwell
    ✓ Metrics endpoint responds
    ✓ Enforcement events logged
    ✓ Dashboard accessible

█ TROUBLESHOOTING

  Q: "asm/types.h: No such file"
  A: Run: sudo ./scripts/fix-asm-symlink.sh

  Q: "Permission denied" when starting daemon
  A: Use: sudo ./bin/dwell-fiber-daemon

  Q: No events in logs
  A: 1. Verify workload is running: cd test && go run workload_generator.go
     2. Check daemon logs: tail -f /tmp/dwell-fiber-test/daemon.log
     3. Try simulation mode first: ./bin/dwell-fiber-daemon --simulate

  Q: Dashboard shows 0 dwell
  A: Run workload generator or switch to simulation mode

  Q: "BPF program not found"
  A: Rebuild: make clean all

  Q: Metrics endpoint not responding
  A: Check port: netstat -tuln | grep 9090
     Kill other daemon: pkill dwell-fiber

█ KEY FILES

  Implementation:
    daemon/bpf_monitor.go       - BPF event processor
    daemon/controller.go         - ADMM algorithm + filtering
    daemon/test_suite.go         - Test scenarios
    test/workload_generator.go   - Synthetic workload
    test/test-suite.sh           - Integration orchestration
  
  Documentation:
    TESTING.md                   - Comprehensive test guide
    TEST_ARCHITECTURE.md         - Architecture overview
    .github/copilot-instructions.md - Development guide
    README.md                    - Project overview

█ PERFORMANCE TARGETS

  BPF overhead:           < 100 μs per syscall
  Event filtering:        < 1 ms
  Enforcement latency:    < 50 ms
  Metrics export:         < 10 ms
  Control loop frequency: 100 ms

█ RELEASE CHECKLIST

  [ ] Build succeeds: make all
  [ ] Coq proofs verify: make verify
  [ ] BPF loads: sudo dmesg | grep dwell
  [ ] Daemon starts: sudo ./bin/dwell-fiber-daemon
  [ ] Workload generates high dwell: cd test && go run workload_generator.go
  [ ] Metrics update: curl http://localhost:9090/metrics
  [ ] Dashboard accessible: firefox http://localhost:9090
  [ ] Enforcement logged: grep "High dwell\|DRY-RUN" /var/log/syslog

█ NEXT STEPS

  1. Build everything:
     $ make clean all
  
  2. Start daemon (simulation):
     $ ./bin/dwell-fiber-daemon --simulate
  
  3. Open dashboard:
     $ firefox http://localhost:9090
  
  4. Generate workload:
     $ cd test && go run workload_generator.go
  
  5. Observe ADMM convergence on dashboard
  
  6. Try real BPF mode:
     $ sudo ./bin/dwell-fiber-daemon

For detailed documentation, see:
  - TESTING.md (comprehensive test guide)
  - TEST_ARCHITECTURE.md (architecture overview)
  - .github/copilot-instructions.md (development guide)

═══════════════════════════════════════════════════════════════════
EOF
