# Changelog

## v0.2.0

### Added
- `jira.issue.comment.list` — read an issue's comments, paginated (`limit`,
  `start_at`, `next_start_at`) and ordered (`order` = `created` / `-created`).

### Changed
- Jira rich-text bodies are now returned as readable **Markdown by default** so
  agents never have to interpret raw ADF. Issue descriptions (`issue.show`,
  `issue.search`, and the issue echoed by create/edit/transition results) and
  comments (`issue.comment.list`, `issue.comment.add`, `issue.comment.edit`)
  render their ADF to Markdown. A new `body_format` input (`markdown` default,
  `adf`, or `both`) selects the representation; `adf`/`both` expose the raw ADF
  under `description_adf` / `body_adf`. Writes are unchanged — they still accept
  Markdown and convert to ADF.
- Bumped the jira plugin manifest version to 0.19.0.

## v0.1.0

- Initial Atlassian (Jira + Confluence) plugins.
