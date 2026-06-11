# Changelog

## v0.2.1

### Fixed
- jsonschema field descriptions containing commas were truncated at the
  first comma when rendered (the tag parser treats commas as option
  separators); affected descriptions are now escaped and render fully.

## v0.2.0

- SDK bump to `fluxplane-plugin` v0.9.0. No operation-surface changes.

## v0.1.0

Initial Ollama plugin: info, model list/show, ps, generate, chat, embed, and
a models datasource.
