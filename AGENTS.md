# AGENTS.md — fluxplane-plugins

Operative rules for agents working in this repo. This is a **published**
monorepo of independently versioned plugin submodules: each plugin has its own
`go.mod`, `CHANGELOG.md`, and git tag (`<plugin>/vX.Y.Z`). The nested `go.work`
exists for local development only. Plugins depend on the SDK
(`github.com/fluxplane/fluxplane-plugin`) and talk to the outside world only
through host capabilities (`pluginbinding` HTTP/secrets/processes) — never
direct `net/http` for operation traffic, never `fluxplane-core`.

## The improvement loop (fix → dogfood → release → upgrade)

Every change — bug fix, new operation, new plugin — walks the same loop.
Don't skip stages; most regressions in this repo were caught by stage 3 or 7,
not stage 2.

1. **Scope** the change to the smallest plugin module(s). One concern per
   commit; unrelated friction discovered on the way becomes an issue
   (stage 8), not a drive-by edit.
2. **Implement + unit tests.** Use `pluginbinding/plugintest`
   (`RunOK`/`RunError`) against `Service` fakes. Result arrays are `[]`,
   never `null`. Mark operation specs honestly: reads `ReadOnly` + risk low,
   writes risk medium+ and non-idempotent, anything returning secret
   material risk high. Bump the manifest `PluginVersion` constant — it is
   versioned independently of the module tag.
3. **Dev-install and dogfood against real systems.** Build into the
   *installed* binary path (not `~/go/bin`):
   `go build -o ~/.fluxplane/plugins/bin/fluxplane-plugin-<name> ./<name>/cmd/fluxplane-plugin-<name>`
   then exercise the changed operations live with
   `fluxplane-plugin operation invoke <name> <op> --input '{…}'`.
   What dogfooding surfaces — confusing errors, input ergonomics, truncation
   on large responses, null-vs-empty — are findings to fix now, not
   annoyances to note.
4. **CHANGELOG** in the same commit: new `## vX.Y.Z` section in the plugin's
   `CHANGELOG.md`, mention the manifest version.
5. **Conventional commit with a body** (`feat(<plugin>): …`, blank line,
   what/why bullets). Use the project bot identity for commits that will be
   published (see `scripts/release/RELEASING.md` in the landscape root).
6. **Release** from the landscape root:
   `bash scripts/release/release-module.sh fluxplane-plugins/<name> vX.Y.Z <name>`
   (per-submodule tag prefix; the script drops workspace replaces, pins
   sibling requires to published versions, verifies with `GOWORK=off`,
   commits, tags, pushes). **New plugins** additionally need a
   `marketplace.json` entry.
7. **Gate the install path** from a neutral cwd (`/tmp`):
   `fluxplane-plugin upgrade` (resolves `@latest` then pins) followed by
   `fluxplane-plugin doctor` — healthy, no not-ok plugins. If the upgrade
   doesn't pick up the new tag, fix the release, don't hand-patch binaries.
8. **Reflect.** Friction encountered while dogfooding that was *not* the work
   item → file a ticket. Durable environment facts → record them where your
   tooling persists notes, not in this repo.

## Live dogfooding safety

- **Reads against production systems are fine; writes are not.** Verify
  write operations only against dev/test targets, or with explicit human
  approval per target.
- Kubernetes: always pass an explicit `--context` / `context` input. Never
  switch the global kubeconfig current-context.
- Credentials resolve **only** from the persisted secret store
  (`fluxplane-plugin auth connect …`) and endpoint refs. Environment
  variables are setup-time hints for `auth connect auto`, never invoke-time
  fallbacks. Never echo credential values — not into terminal output, test
  fixtures, commit messages, or error strings.

## Data hygiene (enforced — published repo)

Nothing in the tree or in new commits may contain:

- **Company-identifying information**: real organization names, internal
  hostnames or domains, cloud account/tenant IDs, customer names,
  ticket-system hosts.
- **Personal information**: real names, usernames, e-mail addresses.
- **Secrets of any kind**, including expired or revoked ones (history is
  forever).

Use neutral fixtures instead: `example.com`/`example.org` domains,
`123456789012`-style account ids, fictional users (`jane` / "Jane Dev"),
generic ticket keys (`DEV-9`). When pasting live output into tests or docs,
sanitize it first — real STS/issue/user payloads are the most common leak
vector. Before committing, a quick self-check:
`grep -rniE "<orgname>|[0-9]{12}|@(?!example)" --include='*.go' --include='*.md' .`
adapted to what you touched.
