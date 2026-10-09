# Contributing

Thanks for taking the time to contribute. This document explains how to get a
change into the project.

## Reporting bugs and requesting features

Open an [issue](https://github.com/glassnode/glassnode-cli/issues). For bugs,
include the output of `gn --version`, your OS, the command you ran and the
error message. `--dry-run` shows the request URL with the API key redacted;
check that nothing you paste contains an API key or token.

Security issues go through [SECURITY.md](SECURITY.md), not the issue tracker.

## Development setup

You need Go 1.24 or later and [golangci-lint](https://golangci-lint.run) v2.

```sh
git clone https://github.com/glassnode/glassnode-cli
cd glassnode-cli
go build -o gn .
go vet ./...
go test ./...
golangci-lint run ./...
```

Tests run against local HTTP servers (`httptest`) and the recorded responses
in `internal/api/testdata/`, so they need no API key and make no network
requests. Running the `gn` binary itself does call the Glassnode API unless
you pass `--dry-run` or point `GLASSNODE_BASE_URL` at a local server.

## Making a change

1. Fork the repository and create a branch from `main`.
2. Make the change, with tests. A behaviour change needs a test that fails
   without it.
3. Run `go vet ./...`, `go test ./...` and `golangci-lint run ./...`.
4. If the change affects the shipped CLI (commands, flags, output, auth,
   install scripts), add a line to the `Unreleased` section of
   [CHANGELOG.md](CHANGELOG.md). Docs-, CI- and tooling-only changes don't
   need an entry.
5. If you add or change a command or flag, update [README.md](README.md) and
   [skills/glassnode-cli/SKILL.md](skills/glassnode-cli/SKILL.md) in the same
   pull request.
6. Open a pull request against `main`. Describe what changes for a user of
   the CLI and why.

CI runs `go vet`, the build and the tests (`test` job) and golangci-lint
(`lint` job) on every pull request and on `main`. A maintainer listed in
[CODEOWNERS](CODEOWNERS) reviews every pull request; once approved, the author
merges.

## Commit messages

Use the imperative mood and a `type: summary` first line, for example
`fix: show the API error message on HTTP 429`. Types are `feat`, `fix`,
`docs`, `test`, `refactor`, `ci` and `chore`. Explain the why in the body when
the summary is not enough.

## Releases

The version comes from the git tag; there is no version file. Maintainers
release as follows:

1. Pick the next version following [Semantic Versioning](https://semver.org/).
   Until 1.0, minor versions may change behaviour and say so in the
   changelog.
2. In a pull request, rename the `Unreleased` section of
   [CHANGELOG.md](CHANGELOG.md) to `vX.Y.Z — YYYY-MM-DD` and add a new, empty
   `Unreleased` section above it. Merge it.
3. Tag the merge commit on `main` with `vX.Y.Z` and push the tag.
4. The [release workflow](.github/workflows/release.yml) runs
   [GoReleaser](https://goreleaser.com) ([.goreleaser.yml](.goreleaser.yml)),
   which builds the `gn` binaries for Linux, macOS and Windows, injects the
   version, and publishes them with `checksums.txt` as a GitHub Release.
5. Replace the release notes with the changelog section for the version.

Only admins and maintainers can create `v*` tags, and tags are immutable: a
published tag is never moved or deleted. A broken release is fixed with a new
patch version.

The install scripts download the latest release, so a release reaches users
of `install.sh` and `install.ps1` as soon as it is published.

## License

By contributing you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE) that covers the project.
