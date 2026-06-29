// Package obs holds observability helpers: Prometheus metrics and HTTP instrumentation.
package obs

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method, route and status.",
	}, []string{"method", "route", "status"})

	duration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency by method and route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	// JobsProcessed counts queue jobs handled by the worker role.
	JobsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "worker_jobs_processed_total",
		Help: "Total jobs processed by the worker.",
	})
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Instrument records request count and latency for every route. It labels by the matched
// route pattern (not the raw path) to keep metric cardinality bounded.
func Instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		duration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
	})
}
