# glassnode-cli: agent instructions

Read [CONTRIBUTING.md](CONTRIBUTING.md) first: setup, commands, commit format,
changelog and release rules all live there. Before opening a PR, run the checks
in its "Making a change" section.

## Don't

- **Don't run `gn` against the live API to test a change.** The `asset`,
  `metric` and `user` commands call the Glassnode API with the user's real
  credentials and spend API credits. Use the tests. `--dry-run` doesn't send the
  request, but it resolves credentials first: it fails without them, and with a
  stored OAuth session it may refresh the token against `auth.glassnode.com` and
  rewrite `~/.gn/config.yaml`. Run it with a temporary `HOME` and
  `GLASSNODE_API_KEY=x`. `GLASSNODE_BASE_URL` redirects API calls only, not
  OAuth.
- **Don't run `gn login`, `gn logout` or `gn config set`** outside a test with
  a temporary `HOME`: they open a browser or rewrite the user's
  `~/.gn/config.yaml`. In tests use `testhelper.WithTempHome`.
- **Don't run `install.sh` or `install.ps1`.** They install the latest release
  system-wide (with `sudo` on Unix). Users pipe them straight from `main`, so a
  change to them ships the moment it merges; they also depend on the archive
  names and `checksums.txt` from [.goreleaser.yml](.goreleaser.yml), so change
  both together.
- **Don't hard-code the version.** It comes from the git tag, injected into
  `internal/version.Version` by GoReleaser; it is `dev` in local builds.
- **Don't edit the recorded API responses in `internal/api/testdata/` to make a
  test pass.** They mirror real API responses.
- **Don't flag the OAuth client ID in `internal/oauth/defaults.go` as a
  leaked secret.** It is a public client ID.
- **Don't add `t.Parallel()` to tests that call
  `oauth.OverrideDefaultsForTesting` or change environment variables.** They
  mutate shared state.
- **Don't let `skills/glassnode-cli/SKILL.md` drift.** It is the agent skill
  that documents `gn` for users' agents; a command or flag change updates it
  and [README.md](README.md) in the same PR.
- **Don't create or move `v*` tags.** Releases are cut by maintainers; see
  CONTRIBUTING.md.
