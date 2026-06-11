# Changelog

## v0.3.1

### Fixed
- jsonschema field descriptions containing commas were truncated at the
  first comma when rendered (the tag parser treats commas as option
  separators); affected descriptions are now escaped and render fully.

## v0.3.0

### Changed
- Runnable JSON Schema `examples` on the ten most-used operations
  (container list/show/logs/stats/exec/run, image list/show/pull, events).
- SDK bump to `fluxplane-plugin` v0.9.0; manifest version 0.19.0.

### Notes
- Invoke-time configuration audit: the daemon address resolves from instance
  config only (`docker_host`), never from the environment. The only
  environment reads are inside `docker.context.list`/`docker.context.show`,
  whose declared purpose is reporting the ambient local Docker CLI context
  (DOCKER_HOST/DOCKER_CONFIG/DOCKER_CONTEXT); those values never route other
  operations' connections.

## v0.2.0

### Changed
- The plugin now talks to the Docker Engine API directly using the official
  Docker SDK, dialing the daemon socket (`unix:///var/run/docker.sock` by
  default, or a `tcp://` host from instance config) through the host
  `conn.dial` capability. This removes the dependency on an app-layer `docker`
  host provider — the plugin performs no direct network IO and works in any host
  that implements the generic conn capability (including the standalone CLI).
- Requires `fluxplane-plugin` v0.3.0.

## v0.1.0

- Initial Docker inspection plugin.
