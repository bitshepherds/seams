# Contributing

This repo is Go. Its tools are pinned in `mise.toml`.

## Set up

1. Install [mise](https://mise.jdx.dev/getting-started.html).
2. Run `mise install` in the repo. It installs the pinned tools, and the git
   hooks.

## Check your work

Run `mise run check`. It's the one command that checks the repo: the gate. CI runs
it on every pull request, and the pre-commit hook runs its fast part,
`mise run fast`, on every commit.

## Work with sheepdog

Plan work in the planning session (`sheepdog plan`), and build each issue in
a stream of its own. See sheepdog's docs.

## Release

Tag a commit on `main` as `vX.Y.Z` and push the tag. The release workflow runs
the gate, then GoReleaser builds `seams` for linux, darwin and windows, on
amd64 and arm64, and publishes them with checksums and a changelog as a GitHub
release. A tag with a suffix, such as `v0.1.0-rc.1`, is a pre-release.

Install a release with `mise use github:bitshepherds/sheepdog-seams@<version>`.
