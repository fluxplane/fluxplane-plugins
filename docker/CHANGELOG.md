# Changelog

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
