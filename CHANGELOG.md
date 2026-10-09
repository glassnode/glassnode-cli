# Changelog

## Unreleased

- `gn metric describe -o table` reports an error when writing the output
  fails instead of ignoring it.

## v0.6.0 — 2026-08-28

- New commands: `gn asset tags`, `gn asset categories`, `gn asset blockchains`
  (each with an optional `--filter` CEL expression) and `gn metric tags`.

## v0.5.0 — 2026-05-11

- Sign in with a Glassnode account: `gn login` (OAuth with PKCE in the
  browser) and `gn logout`. The OAuth access token takes precedence over an
  API key and is refreshed automatically.
- `gn config get` masks `api-key` and the OAuth tokens as `*****-<last4>`.
- The OAuth session fields in the config file are read-only for
  `gn config set`.

## v0.4.4 — 2026-04-09

- New command: `gn user credits` shows the API credit usage of the account.

## v0.4.3 — 2026-03-11

- Requests send `User-Agent: glassnode-cli-<version>`.

## v0.4.2 — 2026-03-11

- Errors no longer print the command's usage text.
- HTTP 429 responses say that the rate limit was exceeded.

## v0.4.1 — 2026-03-10

- The "no API key configured" message shows the correct
  `gn config set api-key=your-key` syntax.

## v0.4.0 — 2026-03-10

- `gn metric list` filters by the metadata query parameters: `--assets`
  (repeatable), `--currency`, `--exchange`, `--format`, `--interval`,
  `--from-exchange`, `--to-exchange`, `--miner`, `--maturity`, `--network`,
  `--period` and `--quote-symbol`.
- Requests send a `User-Agent` header.

## v0.3.3 — 2026-03-10

No changes to the CLI. The agent skill moved to `skills/glassnode-cli/`.

## v0.3.2 — 2026-03-10

No changes to the CLI (release workflow only).

## v0.3.1 — 2026-03-10

No changes to the CLI (release workflow only). No GitHub release was
published for this tag.

## v0.3.0 — 2026-03-10

No changes to the CLI (release workflow only).

## v0.2.2 — 2026-03-10

- `gn --version` reports the release version instead of `dev`.

## v0.2.1 — 2026-03-10

- The Go module path is now `github.com/glassnode/glassnode-cli`.
- `--timestamp-format` defaults to `humanized` and also accepts a Go time
  layout; it applies to table and CSV output, including CSV timestamps.

## v0.2.0 — 2026-03-09

- Table output shows timestamps as `YYYY-MM-DD HH:MM:SS` (UTC) with `TIME`
  and `VALUE` column headers.

## v0.1.0 — 2026-03-06

Initial public release.

- Commands: `gn asset list`, `gn asset describe`, `gn metric list`,
  `gn metric describe`, `gn metric get` (including bulk metrics) and
  `gn config set` / `gn config get`.
- API key from `--api-key`, `GLASSNODE_API_KEY` or `~/.gn/config.yaml`.
- Output as JSON, CSV or a table; `--dry-run` prints the request URL.
- Install scripts for Linux, macOS and Windows that verify the release
  checksum.
