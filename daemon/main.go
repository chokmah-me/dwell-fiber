package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Parse flags
	alpha := flag.Float64("alpha", 0.5, "ADMM step size")
	budget := flag.Float64("budget", 5.0, "Target dwell time budget (seconds)")
	simulate := flag.Bool("simulate", false, "Run in simulation mode")
	port := flag.Int("port", 9090, "Metrics server port")
	flag.Parse()

	fmt.Println("🛡️  Dwell-Fiber Daemon Starting")
	fmt.Printf("   Alpha: %.2f\n", *alpha)
	fmt.Printf("   Budget: %.2f seconds\n", *budget)
	fmt.Printf("   Metrics: http://localhost:%d\n", *port)

	// Create controller
	controller := NewController(*alpha, *budget)

	// Start cleanup routine
	go controller.RunCleanup()

	// Try to load BPF program
	var bpfLoader *BPFMonitor
	var err error

	if !*simulate {
		log.Println("Attempting to load BPF program...")
		bpfLoader, err = NewBPFMonitor(controller)
		if err != nil {
			log.Printf("⚠️  Failed to load BPF: %v", err)
			log.Println("   Falling back to simulation mode")
			*simulate = true
		} else {
			defer bpfLoader.Close()
		}
	}

	// Start metrics server (from your existing metrics.go)
	go StartMetricsServer(*port, controller)

	// Print mode
	if *simulate {
		log.Println("✓ Running in SIMULATION mode")
		log.Println("   Generating synthetic file access patterns...")

		// START SIMULATION GOROUTINE - THIS WAS MISSING!
		go runSimulationLoop(controller)
	} else {
		log.Println("✓ Running with REAL BPF monitoring")
		log.Println("   Tracking actual file dwell times from kernel")
	}

	log.Println("✓ Daemon running (Press Ctrl+C to stop)")

	// Print enforcement info
	fmt.Println("\n📋 Enforcement Status:")
	fmt.Println("   Mode: DRY-RUN (no actual enforcement)")
	fmt.Println("   Throttle threshold: 5.0s")
	fmt.Println("   Kill threshold: 15.0s")
	fmt.Println("   Protected: init, systemd, sshd, NetworkManager, gdm")
	fmt.Println("\n   To enable enforcement: Edit daemon/controller.go")
	fmt.Println("   Set enfConfig.Enabled = true (line 36)")
	fmt.Println("   Set enfConfig.KillEnabled = true (line 40) for kills")

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n✓ Shutting down gracefully...")
}

// runSimulationLoop continuously generates synthetic dwell events
// This simulates file access patterns for testing with better price movement
func runSimulationLoop(controller *Controller) {
	log.Println("🔄 Simulation loop started - generating synthetic events")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	simulationIndex := 0

	for range ticker.C {
		// Generate synthetic dwell time with better balance for price to oscillate
		// 30 second cycle: 8 idle, 8 normal, 7 high, 7 critical
		simulationIndex++
		cycle := simulationIndex % 30 // 30 second cycle for better balance

		var dwell time.Duration
		switch {
		case cycle < 8:
			// Idle phase (1-2 seconds) - 27% of time - ALLOWS PRICE TO DROP
			dwell = time.Duration(1000+int64(cycle*125)) * time.Millisecond
		case cycle < 16:
			// Normal phase (4.5-5.5 seconds) - 27% of time - NEAR BUDGET
			phase := cycle - 8
			dwell = time.Duration(4500+int64(phase*125)) * time.Millisecond
		case cycle < 23:
			// High dwell phase (7 seconds) - 23% of time - ABOVE BUDGET
			dwell = time.Duration(7000) * time.Millisecond
		default:
			// Critical phase (9 seconds, ransomware-like) - 23% of time - WELL ABOVE
			dwell = time.Duration(9000) * time.Millisecond
		}

		// Call controller with simulated event
		pid := 2000 + simulationIndex // Simulated PID
		cmd := "simulated-process"

		controller.HandleCloseEvent(pid, cmd, dwell)
	}
}
