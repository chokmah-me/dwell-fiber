package main

import (
	"sync"
	"testing"

	"github.com/chokmah-me/dwell-fiber/pkg/enforcement"
)

// newTestController creates a Controller for testing without Prometheus registration.
// Constructs the struct directly to avoid MustRegister panics.
func newTestController(alpha, budget, initPrice float64) *Controller {
	return &Controller{
		Alpha:        alpha,
		Budget:       budget,
		currentPrice: initPrice,
		mu:           sync.RWMutex{},
		recentDwells: make([]float64, 0),
		maxRecent:    10,
		enforcer:     enforcement.NewEnforcer(enforcement.DefaultConfig()),
	}
}

func TestCalculateAverageDwell_Empty(t *testing.T) {
	c := newTestController(1.5, 5.0, 0.1)
	avg := c.calculateAverageDwell()
	if avg != 0 {
		t.Errorf("expected 0 for empty dwell list, got %.2f", avg)
	}
}

func TestCalculateAverageDwell_Values(t *testing.T) {
	c := newTestController(1.5, 5.0, 0.1)
	c.recentDwells = []float64{4.0, 5.0, 6.0}

	avg := c.calculateAverageDwell()
	expected := 5.0 // (4 + 5 + 6) / 3
	if avg != expected {
		t.Errorf("expected %.2f, got %.2f", expected, avg)
	}
}

func TestUpdatePrice_Increase(t *testing.T) {
	c := newTestController(1.5, 5.0, 0.1)

	// avgDwell = 6.0 > budget = 5.0, so violation = 1.0
	// newPrice = 0.1 + 1.5 * 1.0 = 1.6
	c.updatePrice(6.0)
	if c.currentPrice != 1.6 {
		t.Errorf("expected 1.6, got %.2f", c.currentPrice)
	}
}

func TestUpdatePrice_Decrease(t *testing.T) {
	c := newTestController(1.5, 5.0, 1.0)

	// avgDwell = 4.0 < budget = 5.0, so violation = -1.0
	// newPrice = 1.0 + 1.5 * (-1.0) = -0.5, clamped to 0
	c.updatePrice(4.0)
	if c.currentPrice != 0 {
		t.Errorf("expected 0 (clamped), got %.2f", c.currentPrice)
	}
}

func TestUpdatePrice_NoNegative(t *testing.T) {
	c := newTestController(2.0, 5.0, 0.5)

	// avgDwell = 1.0 < budget = 5.0, so violation = -4.0
	// rawPrice = 0.5 + 2.0 * (-4.0) = -7.5, clamped to 0
	c.updatePrice(1.0)
	if c.currentPrice < 0 {
		t.Errorf("expected price >= 0, got %.2f", c.currentPrice)
	}
	if c.currentPrice != 0 {
		t.Errorf("expected 0 (clamped), got %.2f", c.currentPrice)
	}
}

func TestGetState_Returns(t *testing.T) {
	c := newTestController(1.5, 5.0, 0.1)
	c.recentDwells = []float64{5.0, 6.0}

	price, dwell, updated, scenario := c.GetState()

	if price != 0.1 {
		t.Errorf("expected price 0.1, got %.2f", price)
	}
	if dwell != 5.5 { // (5 + 6) / 2
		t.Errorf("expected dwell 5.5, got %.2f", dwell)
	}
	if scenario != "real-bpf" {
		t.Errorf("expected scenario 'real-bpf', got %s", scenario)
	}
	if updated.IsZero() {
		t.Errorf("expected non-zero timestamp")
	}
}
