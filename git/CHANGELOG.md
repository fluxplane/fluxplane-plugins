# Changelog

## v0.4.0

Addresses the git section of the #12 field report. Manifest 0.3.0.

### Changed
- **Breaking: operations renamed to the toolchain convention** —
  `git_status`/`git_diff`/`git_add`/`git_commit`/`git_tag`/`git_push` are
  now `git.status`/`git.diff`/`git.add`/`git.commit`/`git.tag`/`git.push`.
- **Non-zero git exits are errors.** `exit_code: 128` ("not a git
  repository") previously returned a *successful* result with "No git
  status output."; every operation now fails loudly with git's stderr.
- **Repositories are targetable**: every operation accepts `repo`
  (`git -C <repo>`); empty keeps the host working directory. Flag-shaped
  paths are rejected.
- SDK bump to `fluxplane-plugin` v0.18.0.


## v0.3.1

### Fixed
- jsonschema field descriptions containing commas were truncated at the
  first comma when rendered (the tag parser treats commas as option
  separators); affected descriptions are now escaped and render fully.

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
