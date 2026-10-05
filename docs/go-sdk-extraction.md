# Go SDK extraction — GN-159

The reusable API implementation is now in the private
[`glassnode-api-go-client`](https://github.com/glassnode/glassnode-api-go-client)
module. The design and TS feature mapping live in that repository's
`docs/design.md`. Tracking: https://glassnode.atlassian.net/browse/GN-159

`internal/api` is a CLI adapter. It retains environment/config resolution,
OAuth refresh, dry-run URL rendering, asset pruning, credit presentation and
historical output shapes. Endpoint HTTP handling, retries and decoding are
provided by the SDK. The SDK defaults to header authentication; the CLI
explicitly retains query API keys to preserve current dry-run and wire behavior.
OAuth uses the SDK token-source API alongside the CLI refresh-on-401 transport.
SDK clients are initialized lazily once per CLI adapter and reused.

GET calls now retry transport/read failures, 429 and 5xx responses twice with
jitter. Context cancellation interrupts calls and backoff. Errors expose SDK
types and redact credentials. Redirects are no longer followed.

The CLI remains public while the SDK is private, so source builds need GitHub
read access. Set `GOPRIVATE` and authenticate Git as described in README.md.
CI/release jobs use `GLASSNODE_SDK_READ_TOKEN`; no credential is committed.
The token must be configured before merging this branch. Do not use
`pull_request_target` to expose it to untrusted fork code.

For local SDK development, create a temporary Go workspace outside either repo
and use both checkouts. If module graph resolution tries to download the
version in the CLI's go.mod, add a version-specific replace to that temporary
workspace. Never commit an absolute filesystem replace into the CLI module.
