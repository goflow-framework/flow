Versioning and release policy
=============================

This document describes the project's versioning and release policy.

Semantic versioning
-------------------
- We follow Semantic Versioning: MAJOR.MINOR.PATCH.
- Increment MAJOR for incompatible API changes.
- Increment MINOR when adding functionality in a backwards-compatible
  manner.
- Increment PATCH for backwards-compatible bug fixes.

API stability and deprecation
-----------------------------
- The `pkg/flow` public API is the stability contract. Any exported symbol
  is considered part of the public API unless explicitly documented as
  internal.
- When deprecating an API, mark it with a comment and provide a replacement
  if possible. Keep the deprecated API for at least one MINOR release cycle
  and document the timeline.

Release checklist
-----------------
1. All tests pass in CI (unit, integration, benchmarks where applicable).
2. Update CHANGELOG.md with notable changes.
3. Bump version in tag and create GitHub release with notes.
