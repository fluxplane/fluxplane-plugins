# Changelog

## v0.3.0

### Fixed
- **`prometheus.targets` no longer dies on large clusters**: it defaults to
  `state=active` — the dropped list carries every discovered-then-
  relabeled-away target and exceeded 240MB on a production cluster,
  truncating mid-JSON into `unexpected end of JSON input`. `state=dropped`/
  `any` remain available explicitly, and a truncated response now names the
  real problem ("response exceeded the 32MB cap…") instead of a JSON parse
  error. Found while live-connecting prometheus for the first time.

## v0.2.1

### Fixed
- jsonschema field descriptions containing commas were truncated at the
  first comma when rendered (the tag parser treats commas as option
  separators); affected descriptions are now escaped and render fully.

## v0.2.0

### Added
- **`prometheus.rules`** — alerting and recording rules from `/api/v1/rules`
  with group, query, state (firing/pending/inactive), `for` duration, labels,
  annotations, health, last error, and active alert count. Filter with
  `type: alert|record`.
- **`prometheus.series`** — series label sets matching PromQL selectors via
  `/api/v1/series` (required `match`, optional start/end window, limit with
  default 100 / max 1000 and an explicit `truncated` flag).
- Runnable JSON Schema `examples` on query, query_range, labels, series,
  targets, and rules. Bumped the plugin manifest to 0.19.0.

### Changed
- **PromQL query results are parsed, not dumped.** `prometheus.query` and
  `prometheus.query_range` now return typed `samples` (vector/scalar/string:
  metric labels + timestamp + value) and `series` (matrix: points over time)
  instead of a raw `results` JSON blob. Values stay strings because Prometheus
  legitimately returns `NaN`/`+Inf`/`-Inf`. Output is bounded: 200 series per
  result and 500 points per series (newest kept), with `truncated` flags at
  both levels; pick `step` so `(end-start)/step` stays under 500.
- **`prometheus.targets` returns typed targets** (job, instance, health,
  scrape pool/URL, last scrape, last error, labels, dropped flag) plus
  `active_count`/`dropped_count` instead of the raw API payload.
- **`prometheus.alerts` returns typed alerts** (name, state, severity,
  active_at, value, labels, annotations) plus `count` instead of the raw API
  payload.
- Datasource records are built from the typed results; query result record
  metadata now carries `{metric, value, timestamp}` / `{metric, points,
  point_count}` shapes instead of raw API objects.
- Requires `fluxplane-plugin` v0.7.0.

Not included (deferred): `/api/v1/metadata` (metric metadata) — add when a
concrete need shows up.

## v0.1.0

- Initial Prometheus plugin.
