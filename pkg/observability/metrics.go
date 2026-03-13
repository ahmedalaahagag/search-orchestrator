package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	Registry           *prometheus.Registry
	RequestsTotal      *prometheus.CounterVec
	SearchDuration     prometheus.Histogram
	StageDuration      *prometheus.HistogramVec
	StageApplied       *prometheus.CounterVec
	ResultCount        prometheus.Histogram
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	factory := promauto.With(reg)

	return &Metrics{
		Registry: reg,

		RequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "search_requests_total",
			Help: "Total number of search requests by status.",
		}, []string{"status"}),

		SearchDuration: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "search_duration_seconds",
			Help:    "Total search latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}),

		StageDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "search_stage_duration_seconds",
			Help:    "Per-stage OpenSearch query duration in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		}, []string{"stage"}),

		StageApplied: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "search_stage_applied_total",
			Help: "Which search stage was used for the final result.",
		}, []string{"stage"}),

		ResultCount: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "search_result_count",
			Help:    "Number of results returned per search.",
			Buckets: []float64{0, 1, 5, 10, 24, 50, 100},
		}),
	}
}
