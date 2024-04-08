package main

import (
	"fmt"
	"math/rand"
	"net/http"
)

// MetricData holds the simulated metrics data
type MetricData struct {
	TotalRequests int
	ErrorCount    int
}

// GenerateMetrics simulates the generation of metrics data.
func GenerateMetrics(data *MetricData) {
	// Simulate total requests
	currentTotalRequests := rand.Intn(100)
	data.TotalRequests += currentTotalRequests // Increment total requests by a random number up to 100

	// Simulate error count based on a 4.5% error rate of new requests
	newErrors := float64(currentTotalRequests) * 0.045
	data.ErrorCount += int(newErrors)
}

// metricsHandler outputs metrics in a format that Prometheus can scrape.
func metricsHandler(data *MetricData) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generate new metrics data
		GenerateMetrics(data)

		// Output metrics in Prometheus exposition format
		fmt.Fprintf(w, "# HELP http_requests_total The total number of HTTP requests.\n")
		fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
		fmt.Fprintf(w, "http_requests_total %d\n", data.TotalRequests)

		fmt.Fprintf(w, "# HELP http_request_errors_total The total number of HTTP request errors.\n")
		fmt.Fprintf(w, "# TYPE http_request_errors_total counter\n")
		fmt.Fprintf(w, "http_request_errors_total %d\n", data.ErrorCount)

		fmt.Fprintf(w, "# HELP http_request_duration_seconds The HTTP request latencies in seconds.\n")
		fmt.Fprintf(w, "# TYPE http_request_duration_seconds histogram\n")
	}
}

func sum(slice []float64) (total float64) {
	for _, v := range slice {
		total += v
	}
	return total
}

func main() {
	// Initialize MetricData
	data := MetricData{}

	http.HandleFunc("/metrics", metricsHandler(&data))

	fmt.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
