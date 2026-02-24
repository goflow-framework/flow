# Docker deploy example

This example demonstrates how to wire a minimal Flow app and build a
small container using a multi-stage Dockerfile.

Build locally:

```bash
# from repo root
docker build -t flow-docker-example -f examples/docker_deploy/Dockerfile .
```

Run:

```bash
docker run -p 8080:8080 flow-docker-example
curl http://localhost:8080/
```

Notes
-----
- The Dockerfile produces a statically linked binary with `CGO_ENABLED=0` and
  copies it into a distroless base image for minimal attack surface.
