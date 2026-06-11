# Changelog

## v0.20.0

### Changed
- `slack.thread` documents its real input contract — provide either `ref`
  (permalink URL or channel:timestamp) **or** `channel`+`ts` together — in
  the operation description and field docs, with runnable examples for both
  forms (fluxplane-plugins#4).
- SDK bump to `fluxplane-plugin` v0.10.0; manifest version 0.20.0.

## v0.19.0

### Added
- `slack.message.list` — read a channel's recent messages
  (conversations.history) with pagination (`limit`, `cursor`, `next_cursor`,
  `has_more`) and time bounds (`oldest`, `latest`). Previously a channel could
  only be searched, not read.

### Changed
- Message text is now rendered to **Markdown by default** so agents never handle
  raw Slack mrkdwn: `*bold*`→`**bold**`, `_italic_`→`*italic*`, `<url|text>`→
  `[text](url)`, `<@U…|name>`→`@name`, `<#C…|name>`→`#name`, HTML entities
  decoded. A `text_format` input (`markdown` default / `mrkdwn` / `both`) selects
  the representation on `slack.message.list` and `slack.thread`. A
  `MarkdownToMrkdwn` converter is also available for write paths.
