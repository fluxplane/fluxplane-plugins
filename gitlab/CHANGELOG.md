# Changelog

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
