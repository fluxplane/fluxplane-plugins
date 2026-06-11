# Changelog

## v0.3.0

### Added
- **`loki.metric`** — LogQL metric queries over a window (query_range matrix
  results): `sum(count_over_time({...}[1d]))` answers "when did this error
  start and at what daily rate" in one call instead of ~20 paged stream
  queries. Inputs: `query`, `since`/`until`, Prometheus-style `step`
  (defaults to ~100 points); typed series with labels and
  timestamp/value samples (fluxplane-plugins#7).
- **Optional HTTP basic auth**: `auth connect loki` accepts `basic_username`
  and `basic_password`; the host composes `Authorization: Basic …` from the
  persisted secret store at call time (requires `fluxplane-plugin` ≥
  v0.13.1). Unauthenticated Lokis keep working — absent secrets are skipped
  (fluxplane-plugins#5). The auth-method description now walks the full
  bootstrap: endpoint save → auth connect → loki.test.

### Fixed
- `since`/`until` accept the literal `now` (the docs always said "defaults
  to now"); unparseable values return a `bad_input` that restates the
  accepted formats (RFC3339 | unix seconds | duration ago | now) instead of
  a raw Go parse error (fluxplane-plugins#7).
- `entries` is always an array — `jq '.entries[]'` works on zero-hit
  results instead of exploding on `null` (fluxplane-plugins#6).

### Changed
- Requires `fluxplane-plugin` v0.13.1. Manifest 0.20.0.

## v0.2.0

### Added
- **`truncated` flag on `loki.query` / `loki.recent_logs` results**: a full
  page (count == limit) signals that more entries likely exist — narrow the
  window or raise the limit.
- Runnable JSON Schema `examples` on `loki.query`, `loki.labels`, and
  `loki.recent_logs`. Manifest 0.19.0.

### Changed
- Requires `fluxplane-plugin` v0.7.0.

## v0.1.0

- Initial Loki plugin.
