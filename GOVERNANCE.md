# Governance (starter)

This repository follows a lightweight governance model. This file is a
starting point and should be extended with contributor roles, decision
making process, and security contact information.

Key points
- Maintain a Code of Conduct (already present in `CODE_OF_CONDUCT.md`).
- Decisions about major API/architecture changes should be discussed and
  recorded in issues and approved by maintainers.
- Security issues should be reported to the security contacts listed here.

Next steps
- Add CODEOWNERS and fill in team members.
- Clarify release sign-off process and maintainers.
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
