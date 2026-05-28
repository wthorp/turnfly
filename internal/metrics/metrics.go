// Package metrics registers and exposes Prometheus metrics for turnfly.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	AllocationsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "turn_allocations_active",
		Help: "Current number of active TURN allocations.",
	})

	AllocationsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "turn_allocations_total",
		Help: "Total number of TURN allocations created.",
	})

	BytesInTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "turn_bytes_in_total",
		Help: "Total bytes received via TURN relay.",
	})

	BytesOutTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "turn_bytes_out_total",
		Help: "Total bytes sent via TURN relay.",
	})

	PacketsDroppedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "turn_packets_dropped_total",
		Help: "Total number of TURN packets dropped.",
	})

	AuthFailuresTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "turn_auth_failures_total",
		Help: "Total number of TURN authentication failures.",
	})

	RegionCandidateWinsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "region_candidate_wins_total",
		Help: "Total number of times each region was chosen as the winning ICE candidate.",
	}, []string{"region"})
)

func Register() {
	prometheus.MustRegister(
		AllocationsActive,
		AllocationsTotal,
		BytesInTotal,
		BytesOutTotal,
		PacketsDroppedTotal,
		AuthFailuresTotal,
		RegionCandidateWinsTotal,
	)
}

// Handler returns an http.Handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
