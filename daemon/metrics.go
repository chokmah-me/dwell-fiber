package main

import (
	"fmt"
	"log"
	"net/http"
)

func StartMetricsServer(port int, ctrl *Controller) {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		price, dwell, updated, scenario := ctrl.GetState()
		
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "# Dwell-Fiber Metrics\n")
		fmt.Fprintf(w, "dwell_fiber_price %.6f\n", price)
		fmt.Fprintf(w, "dwell_fiber_dwell_time %.6f\n", dwell)
		fmt.Fprintf(w, "dwell_fiber_budget %.6f\n", ctrl.Budget)
		fmt.Fprintf(w, "dwell_fiber_violation %.6f\n", dwell-ctrl.Budget)
		fmt.Fprintf(w, "dwell_fiber_last_update %d\n", updated.Unix())
		fmt.Fprintf(w, "# Current scenario: %s\n", scenario)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK\n")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		price, dwell, _, scenario := ctrl.GetState()
		violation := dwell - ctrl.Budget
		
		// Determine status
		status := "🟢 Normal"
		statusColor := "green"
		if dwell > ctrl.Budget+1 {
			status = "🔴 High Dwell"
			statusColor = "red"
		} else if dwell > ctrl.Budget {
			status = "🟡 Warning"
			statusColor = "orange"
		}
		
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Dwell-Fiber Status</title>
    <meta http-equiv="refresh" content="1">
    <style>
        body { font-family: monospace; background: #1e1e1e; color: #d4d4d4; padding: 20px; }
        h1 { color: #4ec9b0; }
        .metric { margin: 15px 0; padding: 10px; background: #2d2d2d; border-radius: 5px; }
        .label { color: #9cdcfe; font-weight: bold; }
        .value { color: #ce9178; font-size: 1.2em; }
        .status { padding: 5px 10px; border-radius: 3px; display: inline-block; }
        .chart { margin: 20px 0; }
        .bar { height: 20px; background: #4ec9b0; display: inline-block; }
        .budget-line { border-left: 2px solid red; height: 30px; display: inline-block; margin-left: -2px; }
    </style>
</head>
<body>
    <h1>🛡️ Dwell-Fiber Real-Time Status</h1>
    
    <div class="metric">
        <span class="label">Status:</span>
        <span class="status" style="background: %s; color: white;">%s</span>
    </div>
    
    <div class="metric">
        <span class="label">Current Scenario:</span>
        <span class="value">%s</span>
    </div>
    
    <div class="metric">
        <span class="label">Dwell Time:</span>
        <span class="value">%.4f seconds</span>
        (budget: %.2f seconds)
    </div>
    
    <div class="chart">
        <div style="width: %.1fpx;" class="bar"></div>
        <div class="budget-line"></div>
        <span style="color: #666;"> ← Budget (5s)</span>
    </div>
    
    <div class="metric">
        <span class="label">Violation:</span>
        <span class="value">%.4f seconds</span>
        %s
    </div>
    
    <div class="metric">
        <span class="label">Current Price:</span>
        <span class="value">%.6f</span>
    </div>
    
    <div class="metric">
        <span class="label">Price Formula:</span>
        <code style="color: #dcdcaa;">price = max(0, %.6f + %.2f × %.4f)</code>
    </div>
    
    <hr style="border-color: #444;">
    
    <div style="color: #666; font-size: 0.9em;">
        <p><strong>Scenarios Demonstrated:</strong></p>
        <ul>
            <li>🟢 Normal: Oscillating around budget</li>
            <li>🔴 Attack: Sustained high dwell (ransomware)</li>
            <li>🟡 Recovery: Gradually improving</li>
            <li>⚪ Idle: Low activity</li>
        </ul>
        <p>Page auto-refreshes every second</p>
        <p><a href="/metrics" style="color: #4ec9b0;">Prometheus Metrics</a> | <a href="/health" style="color: #4ec9b0;">Health Check</a></p>
    </div>
</body>
</html>`, 
			statusColor, status,
			scenario,
			dwell, ctrl.Budget,
			dwell*30,
			violation,
			func() string {
				if violation > 0 {
					return "(OVER BUDGET)"
				}
				return "(under budget)"
			}(),
			price,
			price, ctrl.Alpha, violation,
		)
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("✓ Metrics server listening on %s", addr)
	
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Printf("Metrics server error: %v", err)
	}
}
