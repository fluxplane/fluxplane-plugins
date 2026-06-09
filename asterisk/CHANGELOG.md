# Changelog

## v0.2.0

### Changed
- The AMI ping operation now speaks the Asterisk Manager Interface line protocol
  in-plugin over a TCP stream dialed through the host `conn.dial` capability,
  resolving the endpoint URL and credentials from registered state. This removes
  the app-layer `asterisk` host provider for AMI operations; the plugin performs
  no direct network IO. (Kubernetes-based endpoint discovery still uses the host
  kubernetes provider.)
- Requires `fluxplane-plugin` v0.3.0.

## v0.1.0

- Initial Asterisk plugin.
