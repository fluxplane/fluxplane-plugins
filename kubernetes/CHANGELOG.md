# Changelog

## v0.2.0

### Changed
- Cluster API connections are now dialed through the host `conn.dial` capability
  (`rest.Config.Dial`), so the plugin performs no direct network IO; `client-go`
  still terminates TLS using the kubeconfig CA over the host-dialed stream.
  Kubeconfig parsing and port-forward (via the host process capability) are
  unchanged.
- Requires `fluxplane-plugin` v0.3.0.

## v0.1.0

- Initial Kubernetes plugin.
