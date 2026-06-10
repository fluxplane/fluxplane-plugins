# Changelog

## v0.3.0

### Added
- **Schema introspection operations**: `sql.database.list` (databases; for
  postgres also non-system schemas), `sql.table.list` (tables/views with
  cheap row estimates where the engine keeps statistics — note MySQL 8
  read-only replicas often report none), `sql.table.show` (columns with
  types/nullability/defaults, primary key, grouped foreign keys), and
  `sql.index.list` (per-table or schema-wide, with columns, uniqueness,
  primary, method). information_schema/pg_catalog on mysql/postgres; PRAGMA
  on sqlite with identifiers resolved through a parameterized sqlite_master
  lookup before quoting (PRAGMAs cannot take parameters).
- Runnable JSON Schema `examples` on all five operations.

### Changed
- MySQL information_schema's upper-case result columns are normalized, so
  introspection works on MySQL/Aurora as well as postgres/sqlite.
- SDK bump to `fluxplane-plugin` v0.9.0; manifest version 0.19.0.
- Internal: the endpoint-resolution/read-only-transaction plumbing is shared
  (`withReadOnlySQL`); `sql.query` behavior is unchanged.

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
