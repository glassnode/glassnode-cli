# Go SDK extraction

The reusable API implementation lives in the
[`glassnode-api-go-client`](https://github.com/glassnode/glassnode-api-go-client)
module; its design notes are in that repository's `docs/design.md`.

`internal/api` is a CLI adapter. It retains environment/config resolution,
OAuth login and refresh, dry-run URL rendering, asset pruning, credit
presentation and the historical output shapes. Endpoint HTTP handling,
retries and decoding are provided by the SDK.

- The API key is sent in the `X-Api-Key` header, as the SDK does by default, so
  it stays out of URLs and access logs; `--dry-run` prints the URL without it
  and notes the header on stderr.
- OAuth sessions reach the SDK through its `TokenRefresher` interface: the CLI
  supplies the current token and refreshes it once when the API answers 401.
- SDK clients are initialized lazily once per CLI adapter and reused.
- GET calls retry transport failures, 429 and 5xx responses with jitter and
  honour the API's rate-limit reset. Context cancellation interrupts calls and
  backoff. Errors expose SDK types and redact credentials. Redirects are not
  followed.
- Bulk metrics require `--since`; ranges over the API's per-request limit are
  fetched in windows with `--split`.

For local SDK development, use a temporary `go.work` outside both repositories
with both checkouts; never commit a filesystem `replace` into the CLI module.
