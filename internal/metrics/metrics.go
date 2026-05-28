// Package metrics registers and exposes Prometheus metrics for turnfly.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// AllocationsActive tracks the current number of active TURN allocations.
	AllocationsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "turn_allocations_active",
		Help: "Current number of active TURN allocations.",
	})

	// AllocationsTotal counts the total number of TURN allocations created.
	AllocationsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "turn_allocations_total",
		Help: "Total number of TURN allocations created.",
	})

	// BytesInTotal counts total bytes received via TURN.
	BytesInTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "turn_bytes_in_total",
		Help: "Total bytes received via TURN relay.",
	})

	// BytesOutTotal counts total bytes sent via TURN.
	BytesOutTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "turn_bytes_out_total",
		Help: "Total bytes sent via TURN relay.",
	})

	// PacketsDroppedTotal counts total packets dropped.
	PacketsDroppedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "turn_packets_dropped_total",
		Help: "Total number of TURN packets dropped.",
	})

	// AuthFailuresTotal counts total authentication failures.
	AuthFailuresTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "turn_auth_failures_total",
		Help: "Total number of TURN authentication failures.",
	})
)

// Register registers all turnfly metrics with the default Prometheus registry.
// It panics if a metric collector is already registered (duplicate registration).
func Register() {
	prometheus.MustRegister(
		AllocationsActive,
		AllocationsTotal,
		BytesInTotal,
		BytesOutTotal,
		PacketsDroppedTotal,
		AuthFailuresTotal,
	)
}

// Handler returns an http.Handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
