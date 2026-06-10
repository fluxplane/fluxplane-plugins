# Changelog

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
