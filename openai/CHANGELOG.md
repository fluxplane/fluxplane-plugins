# Changelog

## v0.2.2

### Changed
- **Empty collections always serialize as `[]`** instead of dropping the
  key (fluxplane-plugins#12 sweep; the repo-wide conformance allowlist is
  now empty — the rule is enforced for every plugin).
- Internal: the OpenAI responses wire-decode struct was renamed so the
  repo-wide result-collection conformance sweep stops flagging it.
- SDK bump to `fluxplane-plugin` v0.18.0 (unknown-operation errors carry
  did-you-mean suggestions).


## v0.2.0

- Runnable input example on the vision analyze operation (via the vision
  provider library).
- SDK bump to `fluxplane-plugin` v0.9.0, vision v0.2.0; manifest version
  0.19.0.

## v0.1.0

Initial OpenAI plugin: image generation, vision analysis, model list.
