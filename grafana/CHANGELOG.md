# Changelog

## v0.4.0

Addresses the grafana section of the #12 field report. Manifest 0.21.0.

### Fixed
- **Metric queries through `grafana.loki.query` no longer crash.** Loki
  matrix/vector/scalar responses (numeric timestamps —
  `sum(count_over_time(...))` and friends) are decoded with the shared
  PromQL decoder into `samples`/`series` (`result_type` says which shape
  came back); log queries keep landing in `entries`. Previously the
  streams-only decoder failed with `cannot unmarshal number into … string`.

### Changed
- **Breaking: time ranges are `since`/`until`** on `prometheus.range` and
  `tempo.search` (were `start`/`end`) — one time vocabulary across all
  query plugins.
- Empty collections always serialize (`[]`): dashboard panels/queries,
  prom samples/series, tempo services.
- SDK bump to `fluxplane-plugin` v0.18.0.


## v0.3.0

### Added
- **`grafana.test`** — two-step endpoint probe: `/api/health` (reachability,
  no credentials; reports version + database state) and `/api/org` (do the
  stored credentials work?). Failures carry a `hint` naming the exact
  missing bootstrap step (fluxplane-plugins#5).

### Changed
- The auth method description is now a guided bootstrap: endpoint save →
  mint service-account token (Administration → Service accounts) →
  `auth connect grafana` → `grafana.test` (fluxplane-plugins#5).
- HTTP basic auth (username/password fields) actually works now — it
  requires `fluxplane-plugin` ≥ v0.13.1, where the host first honors
  username/password purposes. Manifest 0.20.0.

## v0.2.1

### Fixed
- jsonschema field descriptions containing commas were truncated at the
  first comma when rendered (the tag parser treats commas as option
  separators); affected descriptions are now escaped and render fully.

## v0.2.0

### Changed
- **Proxy results are parsed, not dumped — `ProxyQueryResult` is gone**
  (pre-1.0, no compatibility shims). Every datasource-proxied operation now
  returns a typed, agent-readable result:
  - `grafana.prometheus.query` / `range` → samples (vector/scalar/string) and
    series (matrix) in the same shape as the prometheus plugin, values kept as
    strings (`NaN`/`±Inf` are legal), bounded at 200 series / 500 points per
    series with `truncated` flags.
  - `grafana.prometheus.rules` → rule groups with type, query, state, `for`,
    health, and active alert counts.
  - `grafana.alerts.active` → typed Alertmanager alerts (name, state,
    severity, starts/ends, silenced_by, inhibited_by, fingerprint, labels,
    annotations); severity/namespace filters work on the typed labels.
  - `grafana.alerts.silences.list/create/delete` → typed silences (id, state,
    matchers, window, created_by, comment), create returns `silence_id`,
    delete returns `deleted`.
  - `grafana.annotation.list/add` → typed annotations with RFC3339 times.
  - `grafana.datasource.health` → `{status, message, source, error}` instead
    of a raw payload (alertmanager fallback included).
  - `grafana.tempo.search` → trace summaries (trace_id, root service/trace,
    start, duration); `grafana.tempo.trace.get` → span summaries (service,
    name, timing, status) root-first, capped at 200 spans with `truncated`,
    plus services list and root duration. Tolerates both `traceID`/`traceId`
    and `batches`/`resourceSpans` shapes.
- **`grafana.loki.query`/`recent_logs` dropped the `raw` payload duplicate**
  and gained a `truncated` flag (full page → more entries likely exist).
- **`alerts.silences.create` accepts `ends_at` as a duration from now**
  (e.g. `2h`) — previously durations meant "ago", which always failed the
  ends-after-starts check.
- Requires `fluxplane-plugin` v0.7.0. Manifest 0.19.0.

### Added
- Runnable JSON Schema `examples` on dashboard.list, annotation.list/add,
  loki.query, loki.recent_logs, prometheus.query/range/rules, alerts.active,
  alerts.silences.create, and tempo.search.

## v0.1.0

- Initial Grafana plugin.
