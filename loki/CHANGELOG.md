# Changelog

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
