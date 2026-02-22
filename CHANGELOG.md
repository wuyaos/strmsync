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
- Normalize server id comparison to preserve connectivity status entries when ids mix string and number types.
- Make server list watching resilient to undefined sources and non-ref inputs.
- Avoid grouping connectivity checks by host/port to prevent credential-specific misclassification.
- Clear connectivity caches when servers are removed to avoid stale growth.
- Trigger connectivity checks immediately on mount to avoid initial unknown state.
- Add a small tolerance window to avoid interval jitter skipping checks.
- Watch server list deeply to react to in-place mutations.
- Tighten connectivity success criteria and standardize id keys; add dev-only failure logs.
- Stop updating connectivity state after unmount and avoid excess workers when queue is small.
- Trigger connectivity checks after list mutations to avoid stale unknown states.
- Reduce reactive overhead for internal connectivity caches.
- Ensure in-flight markers are cleared even during unmount short-circuit.
- Remove redundant per-server in-flight tracking under global polling lock.
- Mark local servers as always reachable in connectivity status.
- Queue a pending refresh when list changes while polling is in-flight.
- Switch polling to recursive timeouts for accurate spacing.
- Extract polling tolerance into a named constant.
- Replace deep watch with a focused signature to limit reactivity overhead.
- Normalize connection status updates and log failures in non-dev environments.

### Changed

- Reduce duplication in server batch operations by extracting a shared executor.
