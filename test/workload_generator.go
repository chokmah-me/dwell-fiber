package main

import (
	"fmt"
	"os"
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

// main allows running this as a standalone tool
func main() {
	outputDir := "/tmp/dwell-fiber-workload"
	wg := NewWorkloadGenerator(outputDir)

	if err := wg.RunFullTestWorkload(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
