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
