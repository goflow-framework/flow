# RequestID generator microbenchmark

Date: 2026-01-12

Summary
-------
This file records a small microbenchmark comparing the fast non-cryptographic
`fastRequestID()` generator against `uuid.New().String()` to demonstrate the
performance improvement from avoiding per-request crypto/syscall work.

Command used
------------
From the repository root:

```bash
go test ./pkg/flow -bench Benchmark -benchmem -run TestFastRequestIDFormat
```

Results (selected)
------------------
- BenchmarkFastRequestID: ~270.5 ns/op, 44 B/op, 3 allocs/op
- BenchmarkUUIDNew:       ~462.5 ns/op, 64 B/op, 2 allocs/op

Interpretation
--------------
- `fastRequestID()` is approximately 1.7x faster in this microbenchmark and
  uses fewer bytes per call. This aligns with prior pprof observations where
  crypto-based UUID generation contributed observable CPU/syscall time.
- The fast generator is intentionally non-cryptographic and intended for
  logging/tracing Request IDs. If you need unpredictable/secure IDs, keep
  using a cryptographic generator in the security-sensitive code paths.

Reproduction notes
------------------
- The benchmark was run on WSL/Linux with Go 1.20+.
- To capture a runtime CPU profile under realistic load, run the instrumented
  `tools/pprof_server` and `tools/pprof_client` as documented in the repo and
  collect a profile via `/debug/pprof/profile` while the client generates
  load.
