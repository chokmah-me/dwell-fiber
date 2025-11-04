#!/bin/bash
# Dwell-Fiber VM Setup & Testing Guide
# Run this on your Ubuntu 25.10 VM to pull the latest changes and start testing

cat << 'EOF'
╔════════════════════════════════════════════════════════════════════════════╗
║         DWELL-FIBER VM SETUP & TESTING - COMPLETE GUIDE                   ║
║                    For Ubuntu 25.10 + dwell-fiber repo                     ║
╚════════════════════════════════════════════════════════════════════════════╝

STEP 1: PREPARE YOUR VM (One-time setup)
═══════════════════════════════════════════════════════════════════════════

Update system packages:
  $ sudo apt-get update
  $ sudo apt-get upgrade -y

Install build dependencies:
  $ sudo apt-get install -y \
      clang llvm libbpf-dev \
      golang-go coq make git \
      netcat-openbsd python3

CRITICAL: Fix Ubuntu 25.10 asm symlink:
  $ sudo ln -sf /usr/include/x86_64-linux-gnu/asm /usr/include/asm

Verify symlink:
  $ ls -la /usr/include/asm

STEP 2: CLONE OR UPDATE REPOSITORY
════════════════════════════════════════════════════════════════════════════

NEW CLONE (if not already cloned):
  $ cd /home/ubuntu
  $ git clone https://github.com/dyb5784/dwell-fiber.git
  $ cd dwell-fiber

EXISTING REPO - Pull latest changes:
  $ cd /home/ubuntu/dwell-fiber
  $ git fetch origin
  $ git pull origin main

Verify you have the test suite:
  $ ls -la daemon/bpf_monitor.go test/test-suite.sh
  $ # Both should exist and be recent

STEP 3: BUILD EVERYTHING
═════════════════════════════════════════════════════════════════════════════

Clean build:
  $ make clean all

Expected output:
  ✓ Building BPF programs...
  ✓ Compiling Coq proofs...
  ✓ Building Go daemon...
  ✓ Binary: bin/dwell-fiber-daemon

Verify build:
  $ ./bin/dwell-fiber-daemon --help
  $ file bin/dwell-fiber-daemon  # Should show: ELF 64-bit

STEP 4: TEST IN SIMULATION MODE (No root required)
════════════════════════════════════════════════════════════════════════════

Terminal 1 - Start daemon in simulation:
  $ ./bin/dwell-fiber-daemon --simulate

Expected output:
  🛡️  Dwell-Fiber Daemon Starting
     Alpha: 0.50
     Budget: 5.00 seconds
     Metrics: http://localhost:9090
  ✓ Metrics server listening on :9090
  ✓ Daemon running (Press Ctrl+C to stop)

Terminal 2 - Monitor metrics:
  $ watch -n 1 'curl -s http://localhost:9090/metrics'

Expected:
  dwell_fiber_price 0.100000
  dwell_fiber_dwell_time 4.500000
  dwell_fiber_budget 5.000000

Terminal 3 - Open dashboard (on your host machine):
  Open Firefox: http://<VM_IP>:9090

Expected:
  - "Dwell-Fiber Real-Time Status" page
  - Green status indicator
  - Dwell time gauge
  - Price display
  - Auto-refreshes every second

Terminal 4 - Generate workload:
  $ cd test && go run workload_generator.go

Expected output:
  🟢 NORMAL WORKLOAD: 5-second dwell operations
    ✓ Normal operation 1/2 (5s dwell)
  🟡 HIGH WORKLOAD: 7-second dwell operations
    ✓ High operation 1/3 (7s dwell, throttle threshold)
  🔴 CRITICAL WORKLOAD: 9+ second dwell operations
    ✓ Critical operation 1/2 (9s dwell, ransomware threshold)
  ⚪ IDLE WORKLOAD: <1 second dwell operations
    ✓ Idle operation 1/2 (<1s dwell)
  ✅ Workload generation complete!

Dashboard should now show:
  - Dwell time: ~5-9 seconds (matches workload)
  - Price: increasing (ADMM responding)
  - Status: 🟡 HIGH or 🔴 HIGH DWELL

Press Ctrl+C on Terminal 1 to stop daemon.

STEP 5: TEST REAL BPF MODE (Requires root + CAP_BPF)
══════════════════════════════════════════════════════════════════════════════

Terminal 1 - Start daemon with real BPF:
  $ sudo ./bin/dwell-fiber-daemon --alpha=0.5 --budget=5.0

Expected output:
  🛡️  Dwell-Fiber Daemon Starting
     Alpha: 0.50
     Budget: 5.00 seconds
  ✓ BPF program loaded
  ✓ Ring buffer reader started
  ✓ Attached to sys_enter_openat
  ✓ Attached to sys_enter_close
  🔍 Event reader started, waiting for events...
  ✓ BPF integration active
  ✓ Running with REAL BPF monitoring
  ✓ Daemon running (Press Ctrl+C to stop)

Terminal 2 - Generate workload:
  $ cd test && go run workload_generator.go

Terminal 3 - Monitor enforcement in real-time:
  $ tail -f /var/log/syslog | grep -i "high dwell\|throttle\|kill"

Expected patterns:
  high dwell: PID=12345 (bash) dwell=7.00s
  high dwell: PID=12346 (bash) dwell=9.00s
  Would throttle PID=12345 (bash) dwell=7.00s

Terminal 4 - Monitor metrics:
  $ watch -n 1 'curl -s http://localhost:9090/metrics | grep dwell_fiber'

Expected:
  dwell_fiber_price 0.350000 [increasing]
  dwell_fiber_dwell_time 6.500000 [tracking workload]

Terminal 5 - Dashboard update:
  Refresh Firefox at http://<VM_IP>:9090
  
Expected:
  - Real dwell time from actual file operations
  - Price curve showing ADMM convergence
  - Status changes based on price

STEP 6: RUN FULL INTEGRATION TEST
═════════════════════════════════════════════════════════════════════════════

Automated test (orchestrates everything):
  $ chmod +x test/test-suite.sh
  $ ./test/test-suite.sh

Expected output (takes ~30 seconds):
  ✓ Build: Success
  ✓ Daemon: Running (PID=1234)
  ✓ Workload: Generated (5 scenarios)
  ✓ Metrics: Available (10 events logged)
  
  📋 Next steps:
    1. Open Firefox: http://localhost:9090
    2. Monitor logs: tail -f /tmp/dwell-fiber-test/daemon.log
    3. Check enforcement: grep "High dwell" /var/log/syslog

STEP 7: VERIFY EVERYTHING WORKS
═════════════════════════════════════════════════════════════════════════════

Checklist:
  [ ] make clean all (succeeds)
  [ ] make verify (Coq proofs verify)
  [ ] ./bin/dwell-fiber-daemon --simulate (starts)
  [ ] curl http://localhost:9090/metrics (responds)
  [ ] firefox http://localhost:9090 (dashboard loads)
  [ ] cd test && go run workload_generator.go (generates files)
  [ ] ./test/test-suite.sh (passes)
  [ ] sudo ./bin/dwell-fiber-daemon (real BPF starts)
  [ ] grep "High dwell" /var/log/syslog (events logged)

If all pass: ✓ SYSTEM WORKING CORRECTLY

STEP 8: COMMON COMMANDS FOR TESTING
═════════════════════════════════════════════════════════════════════════════

Quick restart cycle:
  $ make daemon
  $ ./bin/dwell-fiber-daemon --simulate

Real mode with workload:
  Terminal 1: sudo ./bin/dwell-fiber-daemon
  Terminal 2: cd test && go run workload_generator.go
  Terminal 3: tail -f /var/log/syslog | grep "High dwell"

Monitor specific aspect:
  Metrics: curl http://localhost:9090/metrics
  Enforcement: grep "DRY-RUN" /var/log/syslog
  Dashboard: firefox http://localhost:9090

View test guide:
  $ cat TESTING.md
  $ cat test/QUICK_REFERENCE.sh
  $ cat TROUBLESHOOTING.md

STEP 9: DEBUGGING IF THINGS DON'T WORK
═════════════════════════════════════════════════════════════════════════════

If build fails:
  $ make clean
  $ make daemon  # Try daemon only
  $ # Check: clang -v, llvm-config, libbpf

If daemon won't start:
  $ dmesg | grep -i "dwell\|ebpf" | tail -10
  $ ps aux | grep dwell-fiber-daemon  # Check if already running
  $ pkill dwell-fiber-daemon
  $ # Try simulation mode: ./bin/dwell-fiber-daemon --simulate

If no events appearing:
  $ cd test && go run workload_generator.go  # Generate workload
  $ sudo tail -f /var/log/syslog | grep "high dwell"
  $ # Wait a few seconds for events
  $ # Check if in simulation mode (should disable for real BPF)

If metrics not updating:
  $ curl http://localhost:9090/metrics
  $ netstat -tuln | grep 9090
  $ # Verify daemon is running: ps aux | grep dwell-fiber

If dashboard doesn't load:
  $ curl http://localhost:9090  # Check HTTP response
  $ # Try different browser or hard refresh (Ctrl+Shift+R)

For complete debugging:
  $ cat TROUBLESHOOTING.md

STEP 10: DEMO SCENARIOS
═════════════════════════════════════════════════════════════════════════════

Scenario 1: "Normal System Load"
  Terminal 1: ./bin/dwell-fiber-daemon --simulate
  Terminal 2: cd test && go run workload_generator.go
  Terminal 3: Firefox to dashboard
  
  Observe: Price oscillates, stays near budget

Scenario 2: "Ransomware Attack Simulation"
  Terminal 1: sudo ./bin/dwell-fiber-daemon
  Terminal 2: cd test && go run workload_generator.go -critical=10
  Terminal 3: Firefox to dashboard
  
  Observe: Price rises rapidly, status turns 🔴 RED

Scenario 3: "System Recovery"
  Terminal 1: sudo ./bin/dwell-fiber-daemon
  Terminal 2: # Generate low-dwell workload
           for i in {1..10}; do
             echo "open /tmp/test_$i.txt" > /tmp/test_$i.txt
             sleep 0.5
           done
  Terminal 3: Firefox dashboard
  
  Observe: Price falls to zero, status returns 🟢 GREEN

REFERENCE
═════════════════════════════════════════════════════════════════════════════

Key files:
  TESTING.md                    - Test guide
  TEST_ARCHITECTURE.md          - Architecture details
  TROUBLESHOOTING.md            - Debugging guide
  test/QUICK_REFERENCE.sh       - Quick commands
  daemon/bpf_monitor.go         - BPF event processor [KEY]
  daemon/controller.go          - ADMM algorithm
  test/test-suite.sh            - Integration test
  test/workload_generator.go    - Workload creation

Quick commands:
  make clean all    - Full build
  make verify       - Verify Coq proofs
  make daemon       - Build daemon only
  make test         - Run tests

Port locations:
  Metrics:  http://localhost:9090/metrics
  Dashboard: http://localhost:9090
  Health: http://localhost:9090/health

Git workflow:
  git pull origin main           - Get latest
  git log --oneline             - See commits
  git status                    - Check changes
  git diff daemon/controller.go - See changes

═════════════════════════════════════════════════════════════════════════════

FINAL CHECKLIST

Before testing, ensure:
  [ ] Updated all packages: sudo apt-get update && apt-get upgrade
  [ ] Fixed asm symlink: ls -la /usr/include/asm
  [ ] Cloned/pulled repo: git pull origin main
  [ ] Built everything: make clean all
  [ ] Started daemon: ./bin/dwell-fiber-daemon --simulate
  [ ] Dashboard accessible: firefox http://localhost:9090
  [ ] Workload runs: cd test && go run workload_generator.go
  [ ] Metrics respond: curl http://localhost:9090/metrics

SUCCESS = All items checked ✓

═════════════════════════════════════════════════════════════════════════════

Need help? 
  1. Check TROUBLESHOOTING.md
  2. Run: cat test/QUICK_REFERENCE.sh
  3. Look at: TESTING.md
  4. Review: TEST_ARCHITECTURE.md

Happy testing! 🛡️
EOF
