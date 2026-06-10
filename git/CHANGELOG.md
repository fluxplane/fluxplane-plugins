# Changelog

## v0.3.0

### Changed
- **Typed `data` payload.** `GitResult.Data` is now the typed `GitData` struct
  (process stdout/stderr/exit/truncation flags plus per-operation fields:
  diff mode/truncated/max_bytes, commit/remaining_dirty, tag, push
  remote/refspecs/tags/dry_run) instead of a free-form map — the output
  schema is fully described.
- Runnable JSON Schema `examples` on all six operations.
- SDK bump to `fluxplane-plugin` v0.9.0; manifest version 0.2.0.

## v0.2.0

Hardened argument building (safe ref tokens, force refspecs rejected) and
bounded diff output with truncation signaling.

## v0.1.0

Initial git plugin: status, diff, add, commit, tag, push through the host
process boundary.
