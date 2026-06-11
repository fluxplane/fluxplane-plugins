# Changelog

## v0.22.0

### Fixed
- `slack.file.download` populates its documented top-level `path` with the
  stored blob path (previously only `blob.path` carried it)
  (fluxplane-plugins#8).
- `slack.file.upload` maps Slack's bare `not_in_channel` to an actionable
  error: the token must be a channel member to upload (unlike message.send)
  — run `slack.channel.join` first or try the other role
  (fluxplane-plugins#8).

### Changed
- SDK bump to `fluxplane-plugin` v0.14.0 (its new `blob put PLUGIN FILE`
  closes the local-file → `blob_ref` loop for `slack.file.upload`).
  Manifest 0.22.0.

## v0.21.0

### Added
- Message permalinks resolve to the message: `lookup` with a Slack permalink
  (or `p<ts>` archive URL) now returns the exact message reference
  (`channel:ts`, score 1000) with a ready-to-run `slack.thread` hint instead
  of just the channel (fluxplane-plugins#5).
- `slack.message.send` returns a `permalink` — the shareable
  `https://….slack.com/archives/…` URL of the sent message, resolved
  best-effort after posting (fluxplane-plugins#6).

### Changed
- Read operations honor token roles: `slack.thread` and `slack.message.list`
  accept `role` (`user`/`bot` to force a token; default reads user-then-bot
  with fallback on access errors) and echo the `role` that served the read
  (fluxplane-plugins#5).
- `slack.file.download` falls back bot→user when the bot token cannot see the
  file (`file_not_found`/`channel_not_found`/`missing_scope`), echoes the
  serving `role`, and failed forced-role reads include a "try role: …" hint
  (fluxplane-plugins#5).
- `channel_not_found` and `not_in_channel` joined the fallbackable error
  classes for multi-token reads.
- SDK bump to `fluxplane-plugin` v0.13.0; manifest version 0.21.0.

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
