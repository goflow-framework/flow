# bench-output/

This directory contains local profiling and benchmarking artifacts generated during development (CPU/heap profiles, flamegraphs, profiler logs, etc.). These files are intended for local analysis only and are not required for building or testing the project.

Guidance for collaborators

- These files are large and transient. They are ignored by the repository's `.gitignore` (patterns like `*.prof`, `bench-output/`, `*.svg`, `*.log`). Do not add them to commits.

- If you need to keep a profile for later sharing or documentation, move it out of the repo tree (for example to `~/profiling-archives/flow`):

  mkdir -p ~/profiling-archives/flow
  mv bench-output/*.prof ~/profiling-archives/flow/

- Common tools for analysis:
  - `go tool pprof -http=:6060 <binary> bench-output/cpu.prof` — serves an interactive pprof UI.
  - `pprof -svg <binary> bench-output/heap.prof > heap.svg` — generate flamegraphs/SVGs locally.
  - `speedscope` or `Speedscope.app` can open JSON/Speedscope-compatible profiles.

- If you purposely want to keep a representative profile in the repository for documentation, add it under `docs/` or `examples/` and update `.gitignore` exceptions in a documented commit. Typically this is not recommended because of size and churn.

If you have any preferred conventions for archiving or naming profiles, add them here so the team follows a consistent pattern.
