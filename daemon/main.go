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

var (
	alpha    = flag.Float64("alpha", 0.5, "ADMM step size (0 < α < 2)")
	budget   = flag.Float64("budget", 5.0, "Dwell time budget in seconds")
	port     = flag.Int("port", 9090, "Metrics port")
	simulate = flag.Bool("simulate", false, "Force simulation mode (don't load BPF)")
)

func main() {
	flag.Parse()

	// Check if running as root (needed for BPF)
	if os.Geteuid() != 0 && !*simulate {
		log.Println("⚠️  Warning: Not running as root, BPF loading will fail")
		log.Println("   Run with: sudo ./bin/dwell-fiber-daemon")
		log.Println("   Or use --simulate flag for simulation mode")
	}

	if *alpha <= 0 || *alpha >= 2 {
		log.Fatalf("Invalid alpha: %f (must be 0 < α < 2)", *alpha)
	}

	fmt.Printf("🛡️  Dwell-Fiber Daemon Starting\n")
	fmt.Printf("   Alpha: %.2f\n", *alpha)
	fmt.Printf("   Budget: %.2f seconds\n", *budget)
	fmt.Printf("   Metrics: http://localhost:%d\n", *port)
	fmt.Println()

	ctrl := NewController(*alpha, *budget)

	// Try to load BPF (unless forced simulation)
	if !*simulate {
		if err := ctrl.LoadBPF(); err != nil {
			log.Printf("Failed to initialize BPF: %v", err)
			os.Exit(1)
		}
	} else {
		log.Println("Running in simulation mode (--simulate flag)")
		ctrl.UseRealBPF = false
		ctrl.Scenario = "simulation"
	}

	defer ctrl.Close()

	// Start metrics server
	go StartMetricsServer(*port, ctrl)

	// Control loop
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if ctrl.UseRealBPF {
		log.Println("✓ Running with REAL BPF monitoring")
		log.Println("  Tracking actual file dwell times from kernel")
	} else {
		log.Println("✓ Running in SIMULATION mode")
		log.Println("  Demonstrating 4 scenarios cyclically")
	}

	log.Println("✓ Daemon running (Press Ctrl+C to stop)")

	for {
		select {
		case <-ticker.C:
			ctrl.Update()
		case <-sigCh:
			log.Println("Shutting down...")
			return
		}
	}
}
