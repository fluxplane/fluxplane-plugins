# Changelog

## v0.2.1

### Changed
- **Empty collections always serialize as `[]`** instead of dropping the
  key (fluxplane-plugins#12 sweep; the repo-wide conformance allowlist is
  now empty — the rule is enforced for every plugin).
- SDK bump to `fluxplane-plugin` v0.18.0 (unknown-operation errors carry
  did-you-mean suggestions).


## v0.2.0

- **Typed aggregator surface.** `ProviderSearchFromOperationOutput` now takes
  the typed `SearchOutput` instead of a raw JSON payload; protocol-envelope
  callers decode with the new `DecodeSearchOutput` first. Removes the last
  `json.RawMessage` from the library's public result path.
- Provider search operations declare a runnable JSON Schema `examples` entry,
  surfaced by `fluxplane-plugin operation describe`.
- SDK bump to `fluxplane-plugin` v0.9.0; manifest version 0.19.0.

## v0.1.0

Initial multi-provider web search aggregator library + plugin.
