# Governance

This project follows a lightweight governance model to keep decisions clear
and the community welcoming. This document describes the maintainership,
release, and contribution model for Flow.

## Maintainers and CODEOWNERS

Primary repository maintainers are listed in `.github/CODEOWNERS`. Members
listed there have merge permissions and are expected to review PRs assigned
to them.

## Decision making

Small changes are approved by a maintainer. Larger API or architectural
changes should be proposed as an issue and discussed with maintainers. For
breaking or large design changes, a short design doc should be attached to the
issue and reviewed by at least two maintainers before implementation.

## Releases

- We follow Semantic Versioning (MAJOR.MINOR.PATCH).
- Backwards-incompatible changes bump the MAJOR version and require a deprecation
  strategy and migration notes.
- Minor releases add functionality in a backwards-compatible manner.
- Patch releases are for bug fixes and documentation updates.

Release process (high level):
1. Open a release PR with the changelog and target branch.
2. Cut a release tag (vX.Y.Z) once the release PR is approved and CI passes.
3. Publish binaries/artifacts as needed.

## Contribution etiquette

We aim to be welcoming and inclusive. Follow the `CODE_OF_CONDUCT.md` in
the repo. When in doubt, open an issue to ask before implementing large
changes.
# Governance (quick, low-effort guidelines)

This file contains brief governance notes to set expectations for small-to-medium
contributions. It's intentionally short — use issues/PRs to discuss larger
policy items.

Maintainers

- A small set of maintainers (listed in repository settings) review PRs and
  manage releases.

Decision making

- Small, well-scoped changes: reviewed and merged by maintainers after CI and
  a brief review.
- Larger API or design changes: open an issue or RFC (document describing the
  motivation and alternatives) and reach consensus from maintainers and
  contributors before implementing.

Releases

- Patch/minor releases are cut by maintainers.
- Breaking changes to public APIs (including plugin API major bump) must be
  announced in advance and coordinated with maintainers.

Code of conduct & reporting

- Follow `CODE_OF_CONDUCT.md`; use the issue tracker or private channels to
  report violations.

Security

- For security issues, report privately using the contact in repository
  settings. Maintainers will assess and coordinate disclosure as appropriate.

This file is intentionally brief; expand into a fuller governance model if the
project grows and needs formal committees or working groups.
