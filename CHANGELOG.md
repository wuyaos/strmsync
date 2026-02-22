## [Unreleased]

### Fixed

- Clear route chunk reload marker after successful navigation to allow subsequent auto-retry in the same tab.
- Guard sessionStorage access after navigation to avoid failures when storage is unavailable.
- Harden error response parsing to handle non-object payloads from gateways or proxies.
- Avoid dropping valid server ids when resolving job server names.
- Warn when confirm dialog is invoked concurrently to surface potential races.
- Guard chunk-reload storage marker access during router errors.
- Avoid crashes when server type definitions omit sections or fields.
- Fall back to server id when names are empty or whitespace.
- Allow chunk reload recovery even when sessionStorage is unavailable.

### Changed

- Reduce duplication in server batch operations by extracting a shared executor.
