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

Clean replacement: from an environment inspector to a real read-only AWS
plugin. All AWS traffic is SigV4-signed by aws-sdk-go-v2 and flows through the
host http capability (`pluginbinding.HostHTTPClient`); credentials resolve
exclusively from the persisted secret store (access key, secret key, optional
session token) — never from the environment or `~/.aws` at invoke time.

### Auth model
Setup ingests temporary SSO/STS credentials once:
`aws sso login --profile <p>; eval "$(aws configure export-credentials
--profile <p> --format env)"; fluxplane-plugin auth auto aws
[--instance <account>]`. One plugin instance per AWS account. Expired or
invalid tokens fail with a structured `expired_credentials` error carrying
the refresh flow.

### Operations (all read-only, with runnable examples)
- `aws.test` — STS GetCallerIdentity (account, ARN, latency); wired as the
  auth test operation.
- `aws.ec2.instances` — Name-tag wildcard + state filters, typed records.
- `aws.eks.clusters` — list + describe (version, status, endpoint, VPC).
- `aws.rds.instances` — Aurora/RDS clusters (writer/reader endpoints,
  members) and instances.
- `aws.s3.buckets` / `aws.s3.objects` — bucket and prefix listings with
  continuation tokens.
- `aws.logs.groups` / `aws.logs.tail` / `aws.logs.query` — CloudWatch log
  groups, FilterLogEvents over a window, and bounded Logs Insights queries
  (poll until complete, best-effort StopQuery on timeout).
- `aws.cloudwatch.metrics` — one GetMetricData series over a window.
- `aws.inspect` — the previous environment inspector (its environment reads
  are its declared purpose); context provider and evidence observers kept.
- Datasource `aws.ec2` — EC2 instances searchable by Name tag.

Requires `fluxplane-plugin` v0.9.0.

## v0.2.0

Environment inspection operation, context provider, and evidence observers.
