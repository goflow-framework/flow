# Security Policy

## Supported Versions

Security fixes are applied to the **latest minor release** on the `main` branch.
Older versions do not receive backported security patches.

| Version | Supported |
| ------- | --------- |
| latest (`main`) | ✅ |
| older releases  | ❌ |

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Use GitHub's private vulnerability reporting feature instead:

1. Go to **[Security → Report a vulnerability](https://github.com/goflow-framework/flow/security/advisories/new)**.
2. Fill in a clear description of the issue, steps to reproduce, and the potential impact.
3. Submit the report — it will only be visible to the repository maintainers.

Alternatively, you may e-mail the maintainer directly. Check the repository's
[CODEOWNERS](./CODEOWNERS) file for contact information.

## Response Timeline

| Stage | Target |
| ----- | ------ |
| Acknowledgement | Within **3 business days** |
| Initial assessment | Within **7 business days** |
| Fix / advisory published | Depends on severity — critical issues are prioritised |

We follow responsible disclosure: once a fix is available we will publish a
GitHub Security Advisory and credit the reporter (unless they prefer to remain
anonymous).

## Scope

The following are **in scope**:

- Authentication / session management flaws in `pkg/flow`
- CSRF bypass vulnerabilities
- Injection vulnerabilities (SQL, command, template)
- Insecure default configurations shipped by the framework
- Dependency vulnerabilities that directly affect users of this module

The following are **out of scope**:

- Vulnerabilities in applications *built with* Flow that are not caused by the
  framework itself
- Issues in transitive dependencies that have no practical exploit path through
  this module
- Denial-of-service via resource exhaustion that requires authenticated access

## Disclosure Policy

We ask that reporters:

- Give us reasonable time to investigate and release a fix before public
  disclosure.
- Avoid accessing, modifying, or deleting data that does not belong to them
  during testing.
- Act in good faith and not use vulnerabilities to harm users or the project.

We commit to:

- Respond promptly and keep reporters informed of progress.
- Credit reporters in the published advisory (unless anonymity is requested).
- Not pursue legal action against reporters who follow this policy in good faith.
