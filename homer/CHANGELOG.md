# Changelog

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
