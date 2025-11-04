package main

import (
	"fmt"
	"math/rand"
	"time"
)

// TestScenario represents a test case for the ADMM controller
type TestScenario struct {
	Name        string
	Duration    time.Duration
	DwellTimes  []float64
	Description string
}

// GenerateTestScenarios creates four testing scenarios
func GenerateTestScenarios() []TestScenario {
	return []TestScenario{
		{
			Name:        "🟢 Normal",
			Duration:    30 * time.Second,
			Description: "Dwell oscillates around budget (3-7s), price responds dynamically",
			DwellTimes:  generateNormalScenario(30),
		},
		{
			Name:        "🔴 Attack",
			Duration:    20 * time.Second,
			Description: "Sustained high dwell (7-9s), simulates ransomware, price rises rapidly",
			DwellTimes:  generateAttackScenario(20),
		},
		{
			Name:        "🟡 Recovery",
			Duration:    25 * time.Second,
			Description: "Gradually decreasing dwell (9s→3s), system returns to normal, price decays",
			DwellTimes:  generateRecoveryScenario(25),
		},
		{
			Name:        "⚪ Idle",
			Duration:    15 * time.Second,
			Description: "Low activity (1-2s), price drops to zero, no enforcement",
			DwellTimes:  generateIdleScenario(15),
		},
	}
}

// generateNormalScenario creates dwell times oscillating around budget (5s)
func generateNormalScenario(seconds int) []float64 {
	dwells := make([]float64, 0)
	for i := 0; i < seconds; i++ {
		// Oscillate between 3-7 seconds around budget of 5s
		dwell := 5.0 + (3.0 * float64(i%4-1))
		dwells = append(dwells, dwell)
	}
	return dwells
}

// generateAttackScenario creates sustained high dwell times (ransomware-like)
func generateAttackScenario(seconds int) []float64 {
	dwells := make([]float64, 0)
	for i := 0; i < seconds; i++ {
		// Sustained high dwell: 7-9 seconds
		dwell := 7.0 + rand.Float64()*2.0
		dwells = append(dwells, dwell)
	}
	return dwells
}

// generateRecoveryScenario creates gradually decreasing dwell times
func generateRecoveryScenario(seconds int) []float64 {
	dwells := make([]float64, 0)
	for i := 0; i < seconds; i++ {
		// Gradually recover from 9s back to 3s
		progress := float64(i) / float64(seconds)
		dwell := 9.0 - (6.0 * progress)       // Linear decrease from 9 to 3
		dwell += (rand.Float64() - 0.5) * 0.5 // Add small noise
		dwells = append(dwells, dwell)
	}
	return dwells
}

// generateIdleScenario creates low activity with minimal dwell
func generateIdleScenario(seconds int) []float64 {
	dwells := make([]float64, 0)
	for i := 0; i < seconds; i++ {
		// Low dwell: 1-2 seconds
		dwell := 1.0 + rand.Float64()
		dwells = append(dwells, dwell)
	}
	return dwells
}

// SimulateScenario runs a test scenario on the controller
func SimulateScenario(controller *Controller, scenario TestScenario) {
	fmt.Printf("\n\n═══════════════════════════════════════════════════════════\n")
	fmt.Printf("🧪 Test Scenario: %s\n", scenario.Name)
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("Duration: %v\n", scenario.Duration)
	fmt.Printf("Description: %s\n", scenario.Description)
	fmt.Printf("\n📊 Running simulation...\n")

	// Inject simulated dwell times
	for i, dwell := range scenario.DwellTimes {
		// Simulate a process (bash) with the specified dwell time
		pid := 1000 + i
		cmd := "bash"

		// Call the controller with the simulated dwell
		controller.HandleCloseEvent(pid, cmd, time.Duration(dwell*float64(time.Second)))

		// Print current state every 5 events
		if (i+1)%5 == 0 || i == 0 {
			price, avgDwell, _, _ := controller.GetState()
			fmt.Printf("  [Event %d] PID=%d Dwell=%.2fs | Avg=%.2fs | Price=%.4f\n",
				i+1, pid, dwell, avgDwell, price)
		}

		// Throttle simulation to realistic pace (100ms between events)
		time.Sleep(100 * time.Millisecond)
	}

	// Print final state
	price, avgDwell, updated, _ := controller.GetState()
	fmt.Printf("\n✅ Scenario Complete!\n")
	fmt.Printf("   Final Price: %.4f\n", price)
	fmt.Printf("   Final Avg Dwell: %.2fs\n", avgDwell)
	fmt.Printf("   Last Update: %v\n", updated.Format("15:04:05"))
	fmt.Printf("   Status: %s\n", getScenarioStatus(price, avgDwell))
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
}

// getScenarioStatus returns human-readable status
func getScenarioStatus(price, dwell float64) string {
	if price > 0.5 {
		return "🔴 CRITICAL (Enforcement Active)"
	} else if price > 0.2 {
		return "🟡 HIGH (Throttle threshold)"
	} else if dwell > 5.0 {
		return "🟠 ELEVATED (Above budget)"
	}
	return "🟢 NORMAL (Within budget)"
}

// PrintTestResults prints summary of all test runs
func PrintTestResults(scenarios []TestScenario) {
	fmt.Printf("\n\n╔════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║         DWELL-FIBER TEST SUITE SUMMARY                     ║\n")
	fmt.Printf("╚════════════════════════════════════════════════════════════╝\n\n")

	for i, scenario := range scenarios {
		fmt.Printf("%d. %s\n", i+1, scenario.Name)
		fmt.Printf("   Duration: %v\n", scenario.Duration)
		fmt.Printf("   Events: %d\n", len(scenario.DwellTimes))
		fmt.Printf("   Description: %s\n\n", scenario.Description)
	}

	fmt.Printf("💡 Tips for Testing:\n")
	fmt.Printf("   • Run tests with: go test -v ./daemon\n")
	fmt.Printf("   • Monitor metrics at: http://localhost:9090/metrics\n")
	fmt.Printf("   • Open Firefox dashboard: http://localhost:9090\n")
	fmt.Printf("   • Check enforcement with: grep -i enforce /var/log/syslog\n")
}
