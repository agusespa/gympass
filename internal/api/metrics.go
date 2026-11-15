package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type syncMetrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func newSyncMetrics() *syncMetrics {
	m := &syncMetrics{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gym_tracker_sync_requests_total",
			Help: "Sync requests by HTTP response code.",
		}, []string{"code"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gym_tracker_sync_request_duration_seconds",
			Help:    "Time spent serving sync requests.",
			Buckets: prometheus.DefBuckets,
		}, nil),
	}
	m.registry.MustRegister(m.requests, m.duration)
	// Keep zero-valued series available before the first request.
	for _, code := range []string{"200", "400", "500"} {
		m.requests.WithLabelValues(code)
	}
	m.duration.WithLabelValues()
	return m
}

func (m *syncMetrics) instrument(next http.Handler) http.Handler {
	return promhttp.InstrumentHandlerCounter(m.requests,
		promhttp.InstrumentHandlerDuration(m.duration, next))
}

func (m *syncMetrics) handler() http.Handler {
	metrics := promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		metrics.ServeHTTP(w, r)
	})
}
