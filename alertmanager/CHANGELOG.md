# Changelog

## v0.1.0

Initial Alertmanager plugin (fluxplane incident-troubleshooting stack):

- `alertmanager.test` — readiness + version/cluster status.
- `alertmanager.alerts` — alerts known to Alertmanager (post-routing) with
  active/silenced/inhibited filters and label matchers; bounded, `[]` never
  null.
- `alertmanager.silence.list` — silences with matchers, state, creator,
  comment.
- `alertmanager.silence.create` — matchers + duration/ends_at + mandatory
  comment (risk medium) — the pager-storm tool.
- `alertmanager.silence.delete` — expire a silence by id.
- Endpoint-ref client through the host HTTP capability with optional basic
  auth (persisted secrets only); guided bootstrap in the auth description
  (portforward → endpoint save → auth connect → test).
