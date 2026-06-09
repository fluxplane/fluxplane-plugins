# Changelog

## v0.2.0

### Changed
- Read-only queries now run in-plugin via `database/sql` (MySQL, PostgreSQL,
  SQLite drivers), with networked drivers dialing through the host `conn.dial`
  capability (MySQL `RegisterDialContext`, pgx `DialFunc`). The endpoint URL is
  resolved from the registered endpoint. This removes the app-layer `sql` host
  provider — the wire protocol stays in the driver while only the socket crosses
  the host boundary. SQLite remains file-backed.
- Requires `fluxplane-plugin` v0.3.0.

## v0.1.0

- Initial read-only SQL plugin.
