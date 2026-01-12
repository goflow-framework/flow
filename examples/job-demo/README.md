# Job demo — enqueue jobs for the Flow worker

This small example enqueues a few jobs into Redis using the `pkg/job` queue. Use the included worker CLI (`cmd/flow-worker`) to consume jobs.

Prerequisites

- A Redis server (local or Docker). Example using Docker:

```bash
docker run --rm -p 6379:6379 redis:7
```

Run the worker

From the repository root you can build the worker CLI and run it:

```bash
# build the worker
go build -o bin/flow-worker ./cmd/flow-worker

# run the worker (connects to localhost:6379 by default)
./bin/flow-worker -redis localhost:6379 -ns flowjobs -handlers example,sleep,echo -concurrency 2 -verbose
```

Enqueue jobs (this example)

In another shell run the example program which will enqueue three jobs:

```bash
go run ./examples/job-demo
```

You should see the worker print processing logs for the `example`, `sleep`, and `echo` jobs.

Custom Redis address

Set `REDIS_ADDR` to point to your Redis instance, for example:

```bash
export REDIS_ADDR=redis://my-redis:6379
go run ./examples/job-demo
```

Notes

- The worker supports tuning via CLI flags: `-poll-ms`, `-backoff-ms`, `-jitter-ms`, and `-concurrency`.
- The queue stores immediate jobs in a Redis LIST and scheduled jobs in a ZSET.
- This demo is intentionally minimal — for production-grade job systems consider visibility timeouts, leasing, acknowledgements, and scaling controls.
