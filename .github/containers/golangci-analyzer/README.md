golangci-analyzer image

Purpose
-------
This folder contains a Dockerfile used to build a reproducible analyzer image that
contains a Go toolchain and a golangci-lint binary compiled with that toolchain.
The image also preserves compiled stdlib archives in GOROOT/pkg so analyzer
invocations inside the container see consistent export-data.

Tags / pinning
----------------
We publish and pin images by the Go + golangci-lint combo, for example:
  - ghcr.io/<org>/golangci-analyzer:1.24.11-1.59.0
This lets PR jobs reference a small list of stable images instead of rebuilding
an image on every PR run.

Building and publishing locally (one-off)
----------------------------------------
You can build and push the image locally. When publishing an image for CI,
it is recommended to enable the `make.bash` bootstrap (slow but robust).

# Example (publish with make.bash enabled):
# build and tag
docker build --pull --build-arg USE_MAKE_BASH=1 -t ghcr.io/<ORG>/golangci-analyzer:1.24.11-1.59.0 .
# push (login to ghcr.io first)
docker push ghcr.io/<ORG>/golangci-analyzer:1.24.11-1.59.0

CI / GitHub Actions
-------------------
A scheduled workflow (rebuild-golangci-analyzer.yml) is provided that will
rebuild and republish the pinned analyzer image nightly. The workflow uses the
Docker BuildKit-based `docker/build-push-action` with GitHub Actions cache
support to make incremental builds faster.

Dockerfile notes
----------------
- USE_MAKE_BASH build-arg: the Dockerfile includes a build argument
  `USE_MAKE_BASH` (default `0`). When set to `1`, the Dockerfile will
  perform a full `make.bash` bootstrap if needed to populate the compiler's
  arch-specific package directory. This is slow and only necessary for
  regenerate/publish builds; it is intentionally disabled by default so
  ephemeral builds (e.g., during local development) stay fast.

- Entrypoint: the image includes an entrypoint script that will restore a
  preserved stdlib tarball into GOROOT/pkg at container start if the arch
  directory is missing. This helps runtime containers avoid re-running the
  expensive bootstrap logic.

Recommendations
---------------
- Let the scheduled workflow push the canonical pinned tag (for example
  `1.24.11-1.59.0`) and also a per-run tag (e.g. `...-<run_number>`).
- Keep PR jobs using the pinned tag so they don't rebuild the analyzer image.
- If you need to force a rebuild, trigger the scheduled workflow manually or
  use the workflow_dispatch event.
