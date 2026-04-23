package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/goflow-framework/flow/pkg/job"
)

func main() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	opts := &redis.Options{Addr: addr}
	q := job.NewRedisQueue(opts, "flowjobs")
	defer q.Close()

	ctx := context.Background()

	now := time.Now().UnixNano()

	j1 := &job.Job{ID: fmt.Sprintf("job-%d", now), Type: "example", MaxAttempts: 3}
	if err := q.Enqueue(ctx, j1); err != nil {
		fmt.Fprintf(os.Stderr, "enqueue example: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("enqueued example job id=%s\n", j1.ID)

	p2, _ := json.Marshal(map[string]int{"ms": 200})
	j2 := &job.Job{ID: fmt.Sprintf("job-%d", now+1), Type: "sleep", Payload: p2, MaxAttempts: 3}
	if err := q.Enqueue(ctx, j2); err != nil {
		fmt.Fprintf(os.Stderr, "enqueue sleep: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("enqueued sleep job id=%s payload=%s\n", j2.ID, string(p2))

	p3, _ := json.Marshal(map[string]string{"msg": "hello from job-demo"})
	j3 := &job.Job{ID: fmt.Sprintf("job-%d", now+2), Type: "echo", Payload: p3, MaxAttempts: 1}
	if err := q.Enqueue(ctx, j3); err != nil {
		fmt.Fprintf(os.Stderr, "enqueue echo: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("enqueued echo job id=%s payload=%s\n", j3.ID, string(p3))

	fmt.Println("Done. Start the worker to consume jobs (see README).")
}
