# Changelog

## v0.3.2

### Changed
- **Empty collections always serialize as `[]`** instead of dropping the
  key (fluxplane-plugins#12 sweep; the repo-wide conformance allowlist is
  now empty — the rule is enforced for every plugin).
- SDK bump to `fluxplane-plugin` v0.18.0 (unknown-operation errors carry
  did-you-mean suggestions).


## v0.3.1

### Fixed
- jsonschema field descriptions containing commas were truncated at the
  first comma when rendered (the tag parser treats commas as option
  separators); affected descriptions are now escaped and render fully.

## v0.3.0

### Added
- **Real telephony operations** beyond ping, all speaking the AMI line protocol
  over the host `conn.dial` capability:
  - `asterisk.channel.list` — active channels (live calls) with state, caller
    ID, dialplan position, application, duration, and bridge.
  - `asterisk.peer.list` — PJSIP endpoints (default), SIP or IAX peers, with
    registration address and device status. Zero PJSIP endpoints returns an
    empty list instead of Asterisk's "No endpoints found" error.
  - `asterisk.queue.status` — queue stats (calls, hold/talk time, completed,
    abandoned, service level) with members (status, paused, calls taken, last
    call) and waiting callers.
  - `asterisk.devicestate.list` — device states, filterable by name substring.
  - `asterisk.command` — run an Asterisk CLI command; handles both modern
    `Output:` headers and the pre-13 `Response: Follows` format.
  - `asterisk.call.originate` — place a call (exten+context or application),
    with caller ID, variables, account code, early media, and async control.
  - `asterisk.channel.hangup` — terminate one channel by exact name
    (destructive risk).
- Runnable JSON Schema `examples` on `peer.list`, `command`, `originate`, and
  `hangup`. Bumped the plugin manifest to 0.19.0.

### Fixed
- **`asterisk.ami.ping` no longer misreads unsolicited events as the pong.**
  Modern Asterisk pushes `FullyBooted`/`SuccessfulAuth` events right after
  login, which the old reader consumed as the Ping response, failing the ping.
  All AMI traffic now goes through a session that logs in with `Events: off`
  and matches responses by ActionID. Verified live against Asterisk 22.9.

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
