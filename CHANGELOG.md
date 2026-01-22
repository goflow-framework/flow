# Changelog

## Unreleased

### Added
- Secure default middleware (`SecureHeaders`, `SessionCookieHardening`) and helper `WithSecureDefaults(*App)` to register conservative security headers and session cookie defaults. Added unit tests, an example (`examples/security_demo`) and docs (`docs/security.md`).

### Notes
- Migration: enabling `WithSecureDefaults(app)` is opt-in. To avoid breaking existing setups, `SessionCookieHardening` can be enabled first to append conservative attributes on outgoing `Set-Cookie` headers; migrate session manager settings (call `ApplySecureCookieDefaults()`) once you confirm traffic and clients are compatible.

