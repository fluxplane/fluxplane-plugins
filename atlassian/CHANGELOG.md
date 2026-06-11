# Changelog

## v0.7.0

### Added
- **`web_url` on Jira issue outputs** (fluxplane-plugins#7): `issue.show`,
  `issue.create`, search results, and datasource records carry the human
  `https://<site>.atlassian.net/browse/KEY-123` link alongside the
  api.atlassian.com self URL. The site URL resolves from the new optional
  `site_url` auth field (env hints `ATLASSIAN_SITE_URL`/`JIRA_URL` at
  setup time); installs that connected before the field existed fall back
  to one `accessible-resources` call with the stored token.
- Jira manifest 0.22.0.

## v0.6.1

### Fixed
- jsonschema field descriptions containing commas were truncated at the
  first comma when rendered (the tag parser treats commas as option
  separators); affected descriptions are now escaped and render fully.

## v0.6.0

### Added
- **Confluence: storage-XHTML ↔ Markdown conversion** in `internal/atlassian`
  (`StorageToMarkdown`/`MarkdownToStorage`), mirroring the jira ADF work. Page
  and comment bodies are now rendered to **Markdown by default** so agents
  never handle raw storage XML; `body_format` (`markdown` default / `storage` /
  `both`) selects the representation. Handles headings, lists, tables, code /
  info / note / warning / tip / panel / expand macros, task lists, page/user/
  attachment links (including block-appearance cards), images, emoticons, and
  layouts.
- **`confluence.page.update`** — update a page's title and/or body (Markdown
  preferred), reading the current version and incrementing it automatically.
- **`confluence.page.list`** — list pages by space/title/status with offset
  pagination (`has_more`, `next_page_token`).
- **`confluence.page.comment.list` / `confluence.page.comment.add`** — read and
  write page comments (footer + inline, replies included via `depth=all`) with
  author, timestamps, and Markdown bodies.
- `confluence.page.create` now accepts **`body_markdown`** (converted to
  storage format) in addition to `body_storage`.
- Runnable JSON Schema `examples` on `page.create`, `page.update`, and
  `page.comment.add`. Bumped the confluence plugin manifest to 0.19.0.

### Changed
- `confluence.page.show` folds the nested v1 `space` object into the flat
  `spaceId`/`spaceKey` fields and renders the body per `body_format` instead of
  returning the raw API `body` object.

## v0.5.0

### Added
- Runnable JSON Schema `examples` on the write operations whose input is
  conditional or non-obvious — `issue.transition.run` (the one-of
  transition_id/transition_name/target_status), `issue.attachment.add` (the
  one-of blob_ref/content_bytes), `issue.create`, `issue.edit` (reparenting via
  parent_key), and `issue.comment.add`. `fluxplane-plugin operation describe`
  now shows a copy-pasteable example for these, and local `--dry-run` validation
  treats them as one-of inputs. Bumped the jira plugin manifest to 0.21.0.

## v0.4.0

### Changed
- Jira write verification now uses the shared `pluginbinding.VerifyAppliedWarning`
  convention instead of a hand-rolled check (behavior unchanged). Requires
  `fluxplane-plugin` v0.4.0+.

### Internal
- Added an ADF conformance + fuzz suite (`FuzzMarkdownToADF`/`FuzzADFToMarkdown`)
  in `internal/atlassian`, hardening the Markdown↔ADF converter shared by the
  Jira and Confluence plugins. No behavior change.

## v0.3.0

### Fixed
- **Markdown→ADF: inline code nested in bold/italic/strikethrough no longer
  produces invalid ADF.** `description_markdown` / `body_markdown` such as
  ``**bold with `code` inside**`` previously failed the whole write with
  `400 INVALID_INPUT` because ADF forbids the `code` mark alongside any mark
  except `link`. The converter now strips the incompatible marks, keeping the
  document valid.
- **Jira's field-level error detail is now surfaced.** Failed `issue.create` /
  `issue.edit` (and every other call) now include Jira's `errors` map
  (`field: reason`) and `errorMessages` in the returned error, instead of a bare
  `400: INVALID_INPUT` — no more bisecting the payload by hand.
- **Silent no-op writes are now reported.** After create/edit the issue is
  re-read and the typed fields the caller asked to set (`parent_key`, `summary`,
  `assignee_account_id`) are compared against what actually stuck. Any field
  Jira accepted (HTTP 200) but silently dropped — most notably the
  parent/epic-link in company-managed projects — is named in a loud `warning`
  instead of being masked by `"ok": true`.

### Added
- `jira.issue.edit` accepts `parent_key` to reparent an issue (epic for stories,
  parent for subtasks); previously only `jira.issue.create` had it.
- `jira.issue.show` / `jira.issue.search` now return the issue's `parent`
  (key, summary, status) when set.

### Changed
- `jira.issue.transition.run` description now states that exactly one of
  `transition_id`, `transition_name`, or `target_status` is required (flat
  top-level keys, not a nested `transition` object) and points at
  `jira.issue.transition.list` for discovering valid IDs/names.
- Bumped the jira plugin manifest version to 0.20.0.

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
