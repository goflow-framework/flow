package job

import (
	"context"
	"encoding/json"
	"time"
)

// Job represents a unit of work stored in Redis.
type Job struct {
	ID          string          `json:"id,omitempty"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Attempts    int             `json:"attempts,omitempty"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
	NextRun     int64           `json:"next_run,omitempty"` // unix nanoseconds
	Error       string          `json:"error,omitempty"`
	CreatedAt   int64           `json:"created_at,omitempty"`
}

// Handler executes a job. Returning an error will cause the worker to retry according
// to its configured backoff and the job's MaxAttempts.
type Handler func(ctx context.Context, j *Job) error

// Queue is the minimal interface for enqueuing jobs.
type Queue interface {
	Enqueue(ctx context.Context, j *Job) error
	EnqueueAt(ctx context.Context, j *Job, t time.Time) error
	Close() error
}

// WorkerOptions tunes worker behaviour.
type WorkerOptions struct {
	// Poll interval when checking delayed jobs and queue
	PollInterval time.Duration
	// BackoffBase is the base duration for exponential backoff (delay = BackoffBase * 2^(attempts-1))
	BackoffBase time.Duration
	// Jitter max (milliseconds) to add/subtract from delays
	JitterMillis int
	// Concurrency number of simultaneous job handlers
	Concurrency int
}

// DefaultWorkerOptions returns sensible defaults for tests and small deployments.
func DefaultWorkerOptions() WorkerOptions {
	return WorkerOptions{
		PollInterval: 250 * time.Millisecond,
		BackoffBase:  500 * time.Millisecond,
		JitterMillis: 50,
		Concurrency:  1,
	}
}
