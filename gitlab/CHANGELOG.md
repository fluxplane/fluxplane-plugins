# Changelog

## v0.24.0

Full GitLab release management. Manifest 0.24.0.

### Added
- **Release lifecycle**: `gitlab.release.create` (binds a release to `tag_name`,
  cutting the tag from `ref` — annotated when `tag_message` is set — with
  `name`, `description`, `milestones`, `released_at`, and asset `assets_links`),
  `gitlab.release.show`, `gitlab.release.update` (title/notes/milestones/date),
  and `gitlab.release.delete` (leaves the git tag in place).
- **Changelog**: `gitlab.repository.changelog.generate` builds Markdown release
  notes from the commits between two refs (`version`/`from`/`to`/`date`/
  `trailer`/`config_file`) — drop the result into a release `description`;
  `gitlab.repository.changelog.add` generates and commits a section into the
  repo's changelog file (default `CHANGELOG.md`).
- **Tags**: `gitlab.repository.tag.show` and `gitlab.repository.tag.delete`
  round out the existing `tag.create` / `tag.list`.
- **Release asset links**: `gitlab.release.link.list` / `.create` / `.update` /
  `.delete` manage the download/related-URL links attached to a release.
- Single-release reads/writes return the richer `ReleaseDetail` (web URL,
  milestones, asset links); `gitlab.release.list` keeps the compact shape.

## v0.23.0

Addresses the gitlab section of the #12 field report. Manifest 0.23.0.

### Added
- **`mr.show` accepts `project` + `iid`** alongside `ref` (PROJECT!IID) —
  the same two address forms every sibling mr.* operation takes; runnable
  examples show both.
- **`mr.list` filters by `source_branch` / `target_branch`** — "find the MR
  I just created from this branch" is one call now.

### Changed
- **Breaking: probe renamed** — `gitlab.auth.test` → `gitlab.test`.
- **Empty collections are `[]`, never omitted** across review/compare/tree/
  blob-search/discussion results (matches, commits, files, lines,
  discussions, entries, errors).
- SDK bump to `fluxplane-plugin` v0.18.0.


## v0.22.1

### Fixed
- The advanced-search 400 mapping on `gitlab.search.blobs` now also covers
  group scope ("group-wide code search is unavailable — pass project:")
  (fluxplane-plugins#8).

## v0.22.0

### Added
- **CI/CD + repository reads** (fluxplane-plugins#5): `gitlab.pipeline.list`
  (status/ref/source/username filters), `gitlab.job.list` (per pipeline, with
  stage/status/failure_reason), `gitlab.environment.list` (incl. last
  deployment), `gitlab.deployment.list` (environment/status filters),
  `gitlab.release.list`, `gitlab.repository.tag.list`, and
  `gitlab.repository.commit.list` (ref/file-path/author/since/until filters).
  All bounded with `has_more` truncation flags.
- Merge request records carry `merged_at` and `merged_by`
  (fluxplane-plugins#5).

### Fixed
- `gitlab.repository.file.show` without `ref` resolves the project's default
  branch instead of failing (the files API requires an explicit ref); the
  effective ref is echoed in the result (fluxplane-plugins#5).
- Instance-wide `gitlab.search.blobs` on instances without advanced search
  now returns an actionable message ("pass project: or group:") instead of
  the raw GitLab 400 (fluxplane-plugins#6).
- `gitlab.projects` search ranks name/path matches above description-only
  matches, so the project literally named by the query is first
  (fluxplane-plugins#6).
- SDK bump to `fluxplane-plugin` v0.13.0; manifest version 0.22.0.

## v0.21.0

### Added
- **`gitlab.search.blobs`** — file-content search (scope=blobs) for "where
  does this error string live": project scope (works on every GitLab, with
  optional `ref`), or group/instance scope when the instance has advanced
  search (Elasticsearch/Zoekt). Bounded matches with per-snippet
  `max_data_bytes` caps and `truncated` flags (fluxplane-plugins#4).

### Fixed
- jsonschema field descriptions containing commas were truncated at the
  first comma (the tag parser treats commas as option separators); all
  affected descriptions are now escaped and render fully.

## v0.20.0

End-to-end merge request review workflow (issue #3), with the line-level
position mechanics ported from dex.

### Added
- **`gitlab.mr.changes`** — changed files with bounded unified diffs
  (`max_files` default 50/cap 200 with `truncated`; per-file `max_diff_bytes`
  default 16KB with `diff_truncated`) plus the `diff_refs`
  (base/start/head SHA) line comments anchor to.
- **`gitlab.mr.diff.lines`** — one file's diff parsed into typed lines
  (added/deleted/context with old/new numbers): the exact set of commentable
  lines. Modes: full listing, line+context, regex search.
- **`gitlab.compare`** — commits and bounded diffs between two refs
  (merge-base by default, `straight` for direct comparison).
- **`gitlab.mr.discussion.list` / `gitlab.mr.note.create` /
  `gitlab.mr.discussion.create` / `gitlab.mr.discussion.reply` /
  `gitlab.mr.discussion.resolve`** — discussion threads with resolution state
  and inline positions; line-level comments resolve their SHAs from the
  latest diff version (falling back to the MR's diff_refs) and auto-derive
  `old_line` for context lines from the parsed diff — the combinations
  GitLab accepts without 400s. `discussion.create` supports `dry_run`
  (resolved position + target line in context, nothing posted).
- **`gitlab.mr.update`** — title/description/target_branch/labels and
  close/reopen via `state_event`.
- **`gitlab.repository.tree`** — tree listing at a ref (optionally
  recursive, bounded).
- **`gitlab.repository.file.show`** — file content at a ref, bounded by
  `max_bytes` (default 64KB) with truncation; binary files reported with
  size and blob id instead of dumped.
- **`gitlab.repository.archive`** — repository archive (tar.gz/zip/tar) at a
  ref stored through the host blob capability ("materialize reviewable
  source").
- **`gitlab.project.create`** — create a project, group namespace resolved
  by path.
- Unified diff parser ported verbatim from dex (line classification,
  find-by-old/new line, context windows, regex search) with its test
  fixtures.
- Runnable input examples on all new operations. Manifest version 0.20.0.

## v0.19.0

### Added
- Issue operations — the plugin previously had **zero** issue ops:
  - `gitlab.issue.list` — list/search issues (`project`, `state`, `search`,
    `limit`, `order_by`, `sort`).
  - `gitlab.issue.show` — show one issue by `ref` (`PROJECT#IID`) or
    `project`+`iid`, including description (GitLab-flavored Markdown),
    assignees, and note count.
  - `gitlab.issue.create` — create an issue (title, Markdown description,
    labels, assignees, milestone, confidential).
  - `gitlab.issue.update` — update title/description/assignees, add/remove or
    replace labels, and close/reopen via `state_event`.
  - `gitlab.issue.note.list` / `gitlab.issue.note.create` — read and write
    issue comments (notes), with system-note flagging.
- Runnable JSON Schema `examples` on the issue write operations
  (`issue.create`, `issue.update`, `issue.note.create`) so `operation describe`
  and `--dry-run` show working invocations.

### Changed
- `Issue` output now includes `description`, `assignees`, and
  `user_notes_count`.
