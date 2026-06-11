# Changelog

## v0.4.0

### Added
- **Built-in SIP ladder rendering** (fluxplane-plugins#8): `render: "svg"`
  on `homer.call.show` and `homer.call.analyze` returns `ladder_blob` — an
  SVG sequence diagram with lifelines per host, arrows labeled
  method+offset, inline SDP/media annotations, failure highlighting (>=400,
  BYE, CANCEL in red; 2xx green; 1xx gray), and per-leg labels for merged
  multi-leg flows.

### Changed
- `call.qos` documents `packets` semantics: it is per reporter (cumulative
  counts from that side's RTCP Sender Reports), so a healthy stream can
  read 0 — not proof of missing media (fluxplane-plugins#8).
- `call.list` documents that `end_time`/`duration` span the *discovered*
  messages and may understate long calls; `call.show` fetches the
  authoritative full flow (fluxplane-plugins#8).
- SDK bump to `fluxplane-plugin` v0.14.0. Manifest 0.4.0.

## v0.3.0

### Added
- **`headers` projection on `homer.call.show`** (fluxplane-plugins#7): pass
  `headers: ["X-CID"]` and each flow event carries those SIP header values —
  correlation on custom X-headers without `raw: true` and manual parsing.
- **`number_match: "contains"`** on `homer.search` / `homer.call.list`
  (fluxplane-plugins#7): opt-in broader number matching (digits anywhere via
  LIKE) that catches national formats; the default exact mode now also
  covers the `00`-prefixed variant alongside bare and `+`.

### Fixed
- **`%` wildcards actually match** (fluxplane-plugins#7): `from_user`,
  `to_user`, `ua` filters and the query DSL emit `LIKE` predicates when the
  value contains `%` — previously the documented wildcard literal-matched
  via `=` and could never hit. The smartinput echo shows the effective
  predicate.

### Changed
- Requires `fluxplane-plugin` v0.13.1. Manifest 0.3.0.

## v0.2.0

### Changed
- **Empty searches are diagnosable** (fluxplane-plugins#4): `homer.search`
  and `homer.call.list` results carry a `query` echo with the resolved time
  window (RFC3339 from/to), the effective Homer smartinput, call_id, and
  limit — so "wrong time partition" vs "filters didn't match" vs "this edge
  isn't captured here" can be told apart.
- The `query` DSL field documentation now renders its full field list
  (call_id, cseq, from_user, method, ruri_user, sid, status, to_user, ua,
  user_agent) — commas inside jsonschema descriptions previously truncated
  it at the first field.
- SDK bump to `fluxplane-plugin` v0.10.0.

## v0.1.0

Initial Homer 7 plugin — a port of the original dex homer integration onto the
fluxplane plugin SDK, reshaped for agent use (structured JSON, structured
errors, runnable examples, truncation flags). All HTTP goes through the host
endpoint-ref capability; credentials come exclusively from the persisted
secret store (JWT login per invocation — no env reads, no defaults).

### Operations
- `homer.test` — reachability + authentication probe.
- `homer.search` — SIP message search by number (caller OR callee, ±prefix),
  from/to user, user agent, method, Call-ID, or a query DSL
  (`from_user = '4930%' AND method = 'INVITE'`); flag filters combine via a
  cartesian-product smartinput builder that avoids Homer's unreliable nested
  parentheses. Typed records with RFC3339 times, alias-resolved endpoints,
  and compacted user agents; `truncated` on full pages.
- `homer.call.list` — calls grouped by Call-ID (bounded backward pagination,
  5×200 messages) with caller, callee, derived status (answered/busy/
  cancelled/no answer/failed/ringing), duration, message count, and a
  collapsed IP route.
- `homer.call.show` — ordered flow events (offset, endpoints, method, CSeq,
  compact SDP media annotation like `PCMA :17818`) plus a plain-text ladder;
  `include_raw` attaches full SIP messages.
- `homer.call.qos` — per-stream RTCP aggregation: packets, loss %, jitter
  (clock-rate aware, default 8000 Hz), and an E-model MOS estimate
  (clamped 1.0–4.5).
- `homer.call.analyze` — multi-leg call analysis: seed by Call-ID or
  from/to, fan out on the involved numbers (±30m), correlate legs via a
  shared SIP header value with temporal-overlap filtering, include
  number-matched legs, and report per-leg status/route/header values plus
  the merged flow and ladder.
- `homer.pcap.export` — call messages as a PCAP blob via the host blob store.
- `homer.alias.list` — typed IP/port aliases.
- `homer.calls` search datasource over grouped calls.

In-cluster Homer instances are discovered via the kubernetes plugin
(`kubernetes.endpoint.discover` product `homer`) and reached with
`kubernetes.portforward.start` — this plugin performs no cluster discovery
itself.

Requires `fluxplane-plugin` v0.8.0.
