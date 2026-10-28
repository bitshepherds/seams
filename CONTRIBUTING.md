# Contributing

This repo is Go. Its tools are pinned in `mise.toml`, and `mise.lock` records
each one's checksum and download URL for Linux and macOS. CI installs from the
lockfile, locked, with the mise version its workflows name, so it runs exactly
the tools you do.

## Set up

1. Install [mise](https://mise.jdx.dev/getting-started.html), at least the
   `min_version` in `mise.toml`.
2. Run `mise install` in the repo. It installs the pinned tools, and the git
   hooks.

## Change a tool

Change its version in `mise.toml`, then run `mise lock` to update `mise.lock`
for every platform, and commit both. CI refuses a tool the lockfile doesn't
hold. Tools from npm, pipx and `go install` are locked by version only: the
lockfile holds no checksum for them, or for what they depend on.

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
Each archive comes with an SPDX SBOM, written by Syft. Once the repo is public,
each also gets a build provenance attestation, which `gh attestation verify`
checks.

When a change asks something of a repo that upgrades seams, such as running
`seams render` again, say so in `.github/UPGRADE-NOTES.md`. The release puts
those notes above its changelog. Empty the file, down to its comment, once a
final release is out. Leave it as it is after a pre-release, so that every
pre-release and the final release after them carry the notes.

Install a release with `mise use github:bitshepherds/seams@<version>`.
