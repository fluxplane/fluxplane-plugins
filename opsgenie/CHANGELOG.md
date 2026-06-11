# Changelog

## v0.1.0

Initial Opsgenie plugin (fluxplane incident-troubleshooting stack):

- `opsgenie.test` — validates the stored API key, reports the account.
- `opsgenie.alert.list` — alerts (newest first) via the Opsgenie query
  language; `opsgenie.alert.get` — full alert by id/alias/tiny id.
- `opsgenie.alert.ack` / `opsgenie.alert.close` / `opsgenie.alert.note` —
  the ack loop (risk medium; Opsgenie's async write API echoes
  `request_id`).
- `opsgenie.oncall` — who is on call right now across schedules;
  `opsgenie.schedule.list`.
- Auth: GenieKey API key from the persisted secret store only (`auth
  connect opsgenie --field api_key=…`); EU API host by default, endpoint
  ref to override. Unit-tested against a mock API; live verification
  pending an API key.
