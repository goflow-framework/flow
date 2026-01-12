package job

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisQueue is a small Redis-backed queue using LIST for immediate jobs and ZSET for delayed jobs.
type RedisQueue struct {
	client    *redis.Client
	namespace string
}

// NewRedisQueue constructs a RedisQueue.
func NewRedisQueue(opts *redis.Options, namespace string) *RedisQueue {
	return &RedisQueue{client: redis.NewClient(opts), namespace: namespace}
}

func (rq *RedisQueue) listKey() string   { return rq.namespace + ":queue" }
func (rq *RedisQueue) delayedKey() string { return rq.namespace + ":delayed" }
func (rq *RedisQueue) deadKey() string   { return rq.namespace + ":dead" }

// Enqueue pushes a job to the immediate queue.
func (rq *RedisQueue) Enqueue(ctx context.Context, j *Job) error {
	if j.CreatedAt == 0 {
		j.CreatedAt = time.Now().Unix()
	}
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	// LPUSH so BRPOP will pop the oldest job via RPOP semantics
	return rq.client.LPush(ctx, rq.listKey(), b).Err()
}

// EnqueueAt schedules a job for the future using a ZSET score of unix seconds.
func (rq *RedisQueue) EnqueueAt(ctx context.Context, j *Job, t time.Time) error {
	j.NextRun = t.UnixNano()
	if j.CreatedAt == 0 {
		j.CreatedAt = time.Now().Unix()
	}
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	// use unix nano timestamp as score to allow sub-second scheduling
	return rq.client.ZAdd(ctx, rq.delayedKey(), redis.Z{Score: float64(j.NextRun), Member: b}).Err()
}

// Close closes the underlying client.
func (rq *RedisQueue) Close() error {
	return rq.client.Close()
}

// popImmediate tries to pop a job from the immediate list (non-blocking).
func (rq *RedisQueue) popImmediate(ctx context.Context) (*Job, error) {
	res, err := rq.client.RPop(ctx, rq.listKey()).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	var j Job
	if err := json.Unmarshal(res, &j); err != nil {
		return nil, err
	}
	return &j, nil
}

// moveDue moves jobs from delayed zset to immediate list when score <= now.
func (rq *RedisQueue) moveDue(ctx context.Context) error {
	now := float64(time.Now().UnixNano())
	// ZRANGEBYSCORE with LIMIT to avoid large sweeps
	vals, err := rq.client.ZRangeByScore(ctx, rq.delayedKey(), &redis.ZRangeBy{Min: "-inf", Max: fmt.Sprintf("%f", now), Count: 100}).Result()
	if err != nil {
		return err
	}
	if len(vals) == 0 {
		return nil
	}
	// For each member, remove it from zset and LPUSH to list
	for _, v := range vals {
		// remove one occurrence
		_, err := rq.client.ZRem(ctx, rq.delayedKey(), v).Result()
		if err != nil {
			return err
		}
		// push to list
		if err := rq.client.LPush(ctx, rq.listKey(), v).Err(); err != nil {
			return err
		}
	}
	return nil
}

// Worker is a simple worker processing jobs from a RedisQueue.
type Worker struct {
	queue    *RedisQueue
	handlers map[string]Handler
	opts     WorkerOptions
	stop     chan struct{}
}

// NewWorker constructs a new Worker.
func NewWorker(q *RedisQueue, handlers map[string]Handler, opts WorkerOptions) *Worker {
	if opts.PollInterval == 0 {
		opts = DefaultWorkerOptions()
	}
	return &Worker{queue: q, handlers: handlers, opts: opts, stop: make(chan struct{})}
}

// Start runs the worker loop. It returns when ctx is done or Stop is called.
func (w *Worker) Start(ctx context.Context) error {
	// limited concurrency: spawn N goroutines that will poll the queue
	sem := make(chan struct{}, max(1, w.opts.Concurrency))
	for i := 0; i < max(1, w.opts.Concurrency); i++ {
		sem <- struct{}{}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stop:
			return nil
		default:
			// move due jobs
			_ = w.queue.moveDue(ctx)
			// try to pop immediate job
			j, err := w.queue.popImmediate(ctx)
			if err != nil {
				return err
			}
			if j == nil {
				// nothing to do
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(w.opts.PollInterval):
				}
				continue
			}

			// process job in goroutine respecting concurrency
			<-sem
			go func(job *Job) {
				defer func() { sem <- struct{}{} }()
				_ = w.handleJob(ctx, job)
			}(j)
		}
	}
}

// Stop signals the worker to stop.
func (w *Worker) Stop() {
	close(w.stop)
}

func (w *Worker) handleJob(ctx context.Context, j *Job) error {
	h, ok := w.handlers[j.Type]
	if !ok {
		// no handler: move to dead queue
		_ = w.queue.client.LPush(ctx, w.queue.deadKey(), marshalQuiet(j)).Err()
		return fmt.Errorf("no handler for job type %s", j.Type)
	}
	// call handler
	err := h(ctx, j)
	if err == nil {
		// success; job done
		return nil
	}
	// failure path: increment attempts and reschedule or dead-letter
	j.Attempts++
	j.Error = err.Error()
	if j.MaxAttempts > 0 && j.Attempts >= j.MaxAttempts {
		// dead-letter
		_ = w.queue.client.LPush(ctx, w.queue.deadKey(), marshalQuiet(j)).Err()
		return nil
	}
	// compute backoff: base * 2^(attempts-1) with jitter
	delay := w.opts.BackoffBase * time.Duration(1<<uint(max(0, j.Attempts-1)))
	// add jitter
	if w.opts.JitterMillis > 0 {
		jitter := time.Duration(rand.Intn(w.opts.JitterMillis*2)-w.opts.JitterMillis) * time.Millisecond
		delay += jitter
	}
	next := time.Now().Add(delay)
	_ = w.queue.EnqueueAt(ctx, j, next)
	return nil
}

func marshalQuiet(j *Job) []byte {
	b, _ := json.Marshal(j)
	return b
}

func max(a, b int) int { if a > b { return a } ; return b }
