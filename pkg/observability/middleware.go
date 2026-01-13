package observability

import (
	"net/http"
	"sync"
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

// statusRecorder pool to reduce allocations when wrapping ResponseWriters
// for metrics instrumentation.
var statusRecorderPool sync.Pool

func getStatusRecorder(w http.ResponseWriter) *statusRecorder {
	if v := statusRecorderPool.Get(); v != nil {
		sr := v.(*statusRecorder)
		sr.ResponseWriter = w
		sr.status = http.StatusOK
		return sr
	}
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func putStatusRecorder(s *statusRecorder) {
	// clear the ResponseWriter reference to avoid accidental reuse
	s.ResponseWriter = nil
	s.status = http.StatusOK
	statusRecorderPool.Put(s)
}

// InstrumentHandler returns an http.Handler wrapper that records Prometheus metrics for each request.
func InstrumentHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// capture status code by wrapping ResponseWriter (pooled)
		rw := getStatusRecorder(w)
		next.ServeHTTP(rw, r)
		d := time.Since(start)
		path := r.URL.Path
		method := r.Method
		status := rw.status
		requestCount.WithLabelValues(path, method, http.StatusText(status)).Inc()
		requestDuration.WithLabelValues(path, method).Observe(d.Seconds())
		putStatusRecorder(rw)
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
