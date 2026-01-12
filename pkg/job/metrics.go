package job

import (
	"sync/atomic"
)

var (
	processed int64
	failed    int64
	retried   int64
)

func metricsIncProcessed() { atomic.AddInt64(&processed, 1) }
func metricsIncFailed()    { atomic.AddInt64(&failed, 1) }
func metricsIncRetried()   { atomic.AddInt64(&retried, 1) }

type MetricsSnapshot struct {
	Processed int64
	Failed    int64
	Retried   int64
}

func Metrics() MetricsSnapshot {
	return MetricsSnapshot{
		Processed: atomic.LoadInt64(&processed),
		Failed:    atomic.LoadInt64(&failed),
		Retried:   atomic.LoadInt64(&retried),
	}
}

func resetMetrics() {
	atomic.StoreInt64(&processed, 0)
	atomic.StoreInt64(&failed, 0)
	atomic.StoreInt64(&retried, 0)
}
