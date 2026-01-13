package observability

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "flow_http_requests_total",
		Help: "Total number of HTTP requests processed, labeled by path and method and status.",
	}, []string{"path", "method", "status"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "flow_http_request_duration_seconds",
		Help:    "HTTP request durations in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"path", "method"})
)

// InstrumentHandler returns an http.Handler wrapper that records Prometheus metrics for each request.
func InstrumentHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// capture status code by wrapping ResponseWriter
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		d := time.Since(start)
		path := r.URL.Path
		method := r.Method
		status := rw.status
		requestCount.WithLabelValues(path, method, http.StatusText(status)).Inc()
		requestDuration.WithLabelValues(path, method).Observe(d.Seconds())
	})
}

// statusRecorder wraps http.ResponseWriter to capture status codes.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
