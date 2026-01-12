package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/dministrator/flow/pkg/job"
)

func main() {
	addr := flag.String("redis", "localhost:6379", "redis address")
	ns := flag.String("ns", "flowjobs", "redis namespace prefix")
	flag.Parse()

	opts := &redis.Options{Addr: *addr}
	q := job.NewRedisQueue(opts, *ns)
	defer q.Close()

	handlers := map[string]job.Handler{
		"example": func(ctx context.Context, j *job.Job) error {
			fmt.Println("processing example job", j.ID)
			return nil
		},
	}

	w := job.NewWorker(q, handlers, job.DefaultWorkerOptions())

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		cancel()
	}()

	if err := w.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "worker error: %v\n", err)
		os.Exit(1)
	}

	// give a moment to flush
	time.Sleep(100 * time.Millisecond)
}
