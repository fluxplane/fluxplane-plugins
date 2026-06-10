# Changelog

## v0.4.0

### Added
- **`kubernetes.portforward.list`** — list the managed port-forwards from the
  host process store with target metadata (context, namespace, resource,
  ports), the local URL, and a PID liveness probe (`alive`), filterable by
  namespace/context. Closes the start→list→stop lifecycle: a dead forward is
  now recognizable instead of silently refusing connections. Requires
  `fluxplane-plugin` v0.8.0 (new `process.list` host capability).
- Runnable JSON Schema `examples` on `portforward.start` and
  `portforward.stop`. Bumped the plugin manifest to 0.20.0.

## v0.3.0

### Added
- **`kubernetes.event.list`** — list events (newest first) filterable by
  namespace, involved object name/kind, and `warnings_only`; handles both the
  classic `lastTimestamp` and the modern `eventTime`/series fields. The first
  stop when debugging scheduling, image, or crash issues.
- **`kubernetes.pod.exec`** — run a one-shot command in a pod container
  (WebSocket with SPDY fallback, as kubectl does) returning bounded
  stdout/stderr (1 MiB per stream, truncation flagged), the real exit code,
  and duration. No TTY/stdin; timeout default 30s, max 300s. Note: unlike the
  clientset API calls (which are routed through host `conn.dial`), the exec
  upgrade stream dials directly — client-go's SPDY/websocket round trippers
  ignore `rest.Config.Dial`.
- **`kubernetes.node.list`** — node readiness, roles, abnormal conditions
  (pressure, unschedulable), kubelet version, internal IP, and capacity.
- **`kubernetes.deployment.scale`** — scale via the scale subresource,
  reporting previous and new replica counts.
- **`kubernetes.deployment.restart`** — rolling restart (kubectl rollout
  restart) by bumping the pod-template restart annotation.
- Runnable JSON Schema `examples` on `event.list`, `pod.exec`,
  `deployment.scale`, and `deployment.restart`. Bumped the plugin manifest
  to 0.19.0.

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
