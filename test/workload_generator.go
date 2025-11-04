cd c:\Users\danie\dwell-fiber-1

# Pull the latest changes from GitHub
git pull origin main

# Now push your changes
git push origin main
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// WorkloadGenerator creates synthetic high-dwell file operations
type WorkloadGenerator struct {
	OutputDir string
}

// NewWorkloadGenerator creates a new workload generator
func NewWorkloadGenerator(outputDir string) *WorkloadGenerator {
	return &WorkloadGenerator{OutputDir: outputDir}
}

// GenerateNormalWorkload creates files with ~5s dwell (normal behavior)
func (wg *WorkloadGenerator) GenerateNormalWorkload(count int) error {
	fmt.Println("\n🟢 NORMAL WORKLOAD: 5-second dwell operations")
	for i := 1; i <= count; i++ {
		filename := fmt.Sprintf("%s/normal_%d.txt", wg.OutputDir, i)
		if err := wg.dwellOperation(filename, 5*time.Second); err != nil {
			return err
		}
		fmt.Printf("  ✓ Normal operation %d/%d (5s dwell)\n", i, count)
	}
	return nil
}

// GenerateHighWorkload creates files with ~7s dwell (elevated behavior)
func (wg *WorkloadGenerator) GenerateHighWorkload(count int) error {
	fmt.Println("\n🟡 HIGH WORKLOAD: 7-second dwell operations")
	for i := 1; i <= count; i++ {
		filename := fmt.Sprintf("%s/high_%d.txt", wg.OutputDir, i)
		if err := wg.dwellOperation(filename, 7*time.Second); err != nil {
			return err
		}
		fmt.Printf("  ✓ High operation %d/%d (7s dwell, throttle threshold)\n", i, count)
	}
	return nil
}

// GenerateCriticalWorkload creates files with ~9s dwell (ransomware-like)
func (wg *WorkloadGenerator) GenerateCriticalWorkload(count int) error {
	fmt.Println("\n🔴 CRITICAL WORKLOAD: 9+ second dwell operations (ransomware-like)")
	for i := 1; i <= count; i++ {
		filename := fmt.Sprintf("%s/critical_%d.txt", wg.OutputDir, i)
		if err := wg.dwellOperation(filename, 9*time.Second); err != nil {
			return err
		}
		fmt.Printf("  ✓ Critical operation %d/%d (9s dwell, ransomware threshold)\n", i, count)
	}
	return nil
}

// GenerateIdleWorkload creates files with minimal dwell (idle system)
func (wg *WorkloadGenerator) GenerateIdleWorkload(count int) error {
	fmt.Println("\n⚪ IDLE WORKLOAD: <1 second dwell operations")
	for i := 1; i <= count; i++ {
		filename := fmt.Sprintf("%s/idle_%d.txt", wg.OutputDir, i)
		if err := wg.dwellOperation(filename, 500*time.Millisecond); err != nil {
			return err
		}
		fmt.Printf("  ✓ Idle operation %d/%d (<1s dwell)\n", i, count)
	}
	return nil
}

// GenerateVariedWorkload creates a mix of all workload types
func (wg *WorkloadGenerator) GenerateVariedWorkload() error {
	fmt.Println("\n🎯 VARIED WORKLOAD: Mixed scenarios")
	scenarios := []struct {
		name     string
		duration time.Duration
		count    int
	}{
		{"Idle (0.5s)", 500 * time.Millisecond, 2},
		{"Normal (5s)", 5 * time.Second, 2},
		{"High (7s)", 7 * time.Second, 3},
		{"Critical (9s)", 9 * time.Second, 2},
	}

	opNum := 0
	for _, scenario := range scenarios {
		for i := 0; i < scenario.count; i++ {
			opNum++
			filename := fmt.Sprintf("%s/varied_%d.txt", wg.OutputDir, opNum)
			fmt.Printf("  [%d] %s", opNum, scenario.name)
			if err := wg.dwellOperation(filename, scenario.duration); err != nil {
				return err
			}
			fmt.Println(" ✓")
		}
	}
	return nil
}

// dwellOperation performs a single file operation with specified dwell time
func (wg *WorkloadGenerator) dwellOperation(filename string, dwell time.Duration) error {
	// Create parent directory if needed
	os.MkdirAll(wg.OutputDir, 0755)

	// Open file for writing
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	// Write initial data
	fmt.Fprintf(file, "Operation started at %s\n", time.Now().Format(time.RFC3339Nano))

	// Keep file open for the specified dwell time
	startTime := time.Now()
	ticker := time.NewTicker(dwell / 2)
	defer ticker.Stop()

	for range ticker.C {
		if time.Since(startTime) >= dwell {
			break
		}
		fmt.Fprintf(file, "Tick at %s\n", time.Now().Format(time.RFC3339Nano))
		file.Sync()
	}

	// Write final data
	fmt.Fprintf(file, "Operation completed at %s (dwell: %.2fs)\n",
		time.Now().Format(time.RFC3339Nano), time.Since(startTime).Seconds())

	return file.Close()
}

// RunFullTestWorkload runs all workload types sequentially
func (wg *WorkloadGenerator) RunFullTestWorkload() error {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║         DWELL-FIBER WORKLOAD GENERATOR                     ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	fmt.Printf("\nWorkload directory: %s\n", wg.OutputDir)

	// Ensure directory exists
	os.MkdirAll(wg.OutputDir, 0755)

	// Run workloads
	if err := wg.GenerateIdleWorkload(2); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	if err := wg.GenerateNormalWorkload(2); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	if err := wg.GenerateHighWorkload(3); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	if err := wg.GenerateCriticalWorkload(2); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)

	if err := wg.GenerateVariedWorkload(); err != nil {
		return err
	}

	fmt.Println("\n✅ Workload generation complete!")
	fmt.Println("\n📊 Expected behavior:")
	fmt.Println("  • Idle ops (<1s): No enforcement")
	fmt.Println("  • Normal ops (5s): Price near budget")
	fmt.Println("  • High ops (7s): Should trigger throttling 🐌")
	fmt.Println("  • Critical ops (9s): Should trigger killing 💀")
	fmt.Println("\n📈 Monitor at: http://localhost:9090")
	fmt.Println("📋 Check logs: grep -i 'High dwell\\|Throttle\\|Kill' <daemon-log>")

	return nil
}

// RunContinuousWorkload creates persistent processes with long-held files
// This allows enforcement to actually catch and throttle/kill them
func (wg *WorkloadGenerator) RunContinuousWorkload(duration time.Duration) error {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║    CONTINUOUS WORKLOAD: Long-running enforcement test      ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	fmt.Printf("\nDuration: %v\n", duration)
	fmt.Printf("Expected: Daemon will throttle/kill this process\n\n")

	os.MkdirAll(wg.OutputDir, 0755)

	// Create a file and keep it open for the specified duration
	filename := fmt.Sprintf("%s/continuous_high_dwell.txt", wg.OutputDir)
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create continuous file: %w", err)
	}
	defer file.Close()

	fmt.Printf("📂 File: %s\n", filename)
	fmt.Printf("⏰ Process will hold file open for %v\n", duration)
	fmt.Printf("🎯 Watch for enforcement actions on PID=%d\n\n", os.Getpid())

	// Write initial marker
	fmt.Fprintf(file, "Continuous workload started at %s\n", time.Now().Format(time.RFC3339Nano))
	fmt.Fprintf(file, "Expected enforcement:\n")
	fmt.Fprintf(file, "  - 0-5s: No enforcement (below throttle threshold)\n")
	fmt.Fprintf(file, "  - 5-15s: THROTTLE applied (20%% CPU quota)\n")
	fmt.Fprintf(file, "  - 15s+: KILL signal sent (ransomware defense)\n\n")
	file.Sync()

	// Keep file open and write periodic updates
	start := time.Now()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(start)
			if elapsed >= duration {
				fmt.Printf("✅ Continuous workload complete (%.2fs held)\n", elapsed.Seconds())
				fmt.Fprintf(file, "Continuous workload completed at %s (dwell: %.2fs)\n",
					time.Now().Format(time.RFC3339Nano), elapsed.Seconds())
				return nil
			}
			fmt.Fprintf(file, "Tick: %.2f seconds elapsed\n", elapsed.Seconds())
			file.Sync()
			fmt.Printf("  📝 %.2fs: File still open (PID=%d)\n", elapsed.Seconds(), os.Getpid())
		}
	}
}

// RunAttackSimulation creates realistic ransomware-like behavior:
// Multiple processes holding files open for increasingly long periods
func (wg *WorkloadGenerator) RunAttackSimulation() error {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║       RANSOMWARE ATTACK SIMULATION                          ║")
	fmt.Println("║  Creating multiple processes with escalating dwell times    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	os.MkdirAll(wg.OutputDir, 0755)

	scenarios := []struct {
		name     string
		dwell    time.Duration
		numFiles int
	}{
		{"Stage 1: Reconnaissance (5s dwell)", 5 * time.Second, 2},
		{"Stage 2: Initial Encryption (7s dwell)", 7 * time.Second, 3},
		{"Stage 3: Heavy Encryption (10s dwell)", 10 * time.Second, 4},
		{"Stage 4: Critical Encryption (15s dwell)", 15 * time.Second, 2},
	}

	for stageIdx, scenario := range scenarios {
		fmt.Printf("\n%s\n", scenario.name)
		fmt.Println(strings.Repeat("=", 60))

		for i := 1; i <= scenario.numFiles; i++ {
			filename := fmt.Sprintf("%s/attack_stage%d_file%d.txt", wg.OutputDir, stageIdx+1, i)
			fmt.Printf("  [%d/%d] Holding file for %.1fs: %s\n",
				i, scenario.numFiles, scenario.dwell.Seconds(), filename)

			if err := wg.dwellOperation(filename, scenario.dwell); err != nil {
				return err
			}
			fmt.Printf("       ✓ Complete\n")
		}

		// Brief pause between stages
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✅ Attack simulation complete!")
	fmt.Println("\n📊 Enforcement Summary:")
	fmt.Println("  • Stage 1 (5s): Should NOT be throttled (at budget)")
	fmt.Println("  • Stage 2 (7s): THROTTLED (5s+ threshold)")
	fmt.Println("  • Stage 3 (10s): THROTTLED (5s+ threshold)")
	fmt.Println("  • Stage 4 (15s): KILLED (ransomware defense)")

	return nil
}

// RunInteractiveMenu provides an interactive mode for testing
func (wg *WorkloadGenerator) RunInteractiveMenu() error {
	fmt.Println("\n╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║     DWELL-FIBER WORKLOAD GENERATOR - INTERACTIVE MODE      ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	fmt.Println("\nAvailable test modes:")
	fmt.Println("  1. Full Test Suite (idle/normal/high/critical/varied)")
	fmt.Println("  2. Continuous Workload (30s with enforcement)")
	fmt.Println("  3. Attack Simulation (escalating ransomware pattern)")
	fmt.Println()

	// Default to option 1 if not interactive
	mode := flag.Int("mode", 1, "Test mode (1=full, 2=continuous, 3=attack)")
	duration := flag.Duration("duration", 30*time.Second, "Duration for continuous mode")
	flag.Parse()

	switch *mode {
	case 1:
		return wg.RunFullTestWorkload()
	case 2:
		return wg.RunContinuousWorkload(*duration)
	case 3:
		return wg.RunAttackSimulation()
	default:
		return fmt.Errorf("unknown mode: %d", *mode)
	}
}

// main allows running this as a standalone tool
func main() {
	outputDir := "/tmp/dwell-fiber-workload"
	wg := NewWorkloadGenerator(outputDir)

	// Parse command-line flags
	mode := flag.Int("mode", 1, "Test mode (1=full, 2=continuous, 3=attack)")
	duration := flag.Duration("duration", 30*time.Second, "Duration for continuous mode")
	continuous := flag.Bool("continuous", false, "Run continuous workload (alias for -mode=2)")
	attack := flag.Bool("attack", false, "Run attack simulation (alias for -mode=3)")
	flag.Parse()

	// Handle flag aliases
	if *continuous {
		*mode = 2
	}
	if *attack {
		*mode = 3
	}

	// Run selected mode
	var err error
	switch *mode {
	case 1:
		err = wg.RunFullTestWorkload()
	case 2:
		err = wg.RunContinuousWorkload(*duration)
	case 3:
		err = wg.RunAttackSimulation()
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %d\n", *mode)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
