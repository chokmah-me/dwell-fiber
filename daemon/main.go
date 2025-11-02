package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
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
		log.Println("   Use --simulate=false for real BPF monitoring")
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
