# seams

`seams` finds a system's seams, the contract boundaries where one part meets
another, and keeps a registry of them. A seam is an HTTP or gRPC API, an
event or a queue, a shared file, config or table, a library or a CLI. Each
has a provider, its consumers and an audience, internal or public.

The registry lives in the repo, in `.sheepdog/seams/registry.json`. From it,
`seams render` writes `docs/SEAMS.md`, the register: every seam, with a
diagram of the components and the seams between them, and how well each is
covered. See this repo's own [register](docs/SEAMS.md), and
[how to read it](docs/READING-THE-REGISTER.md), a guide in plain words for
anyone who isn't a developer.

## Why sheepdog-seams

In a distributed system, much of what goes wrong goes wrong between
components, not inside them. Each side of a seam is built and tested by its
own team, against its own idea of the contract. When those ideas drift
apart, both sides' tests still pass. A field is renamed, an event changes
shape, a status code takes on a new meaning, and nobody finds out until the
two meet in production.

Drift like that is hard to catch because no one owns the gap. The contract
is often implicit, written down nowhere, or in a file only one side reads.
Each change looks safe from where it's made. The seams nobody thought of as
seams, a shared table, a config file, a queue another team reads, have no
contract at all. So the failures turn up late, cost the most to fix, and
are blamed on whichever side changed last.

The fix is well known: make each contract explicit, version it, check each
change against it, and test both sides against it, consumer and provider.
But a team can only do that for the seams it knows about. `seams` finds
them, records who provides and consumes each one, and keeps a register that
shows how well each is covered, gaps first. So the team can apply good
practice to every seam, not only the ones it happens to remember.

## Install

```sh
mise use github:bitshepherds/sheepdog-seams@<version>
```

Or download an archive for your platform from the GitHub releases.

## Use

```sh
seams detect                    # propose the repo's seams, with evidence
seams add http:billing/invoices --consumer web --audience internal
seams add cli:tool/registry --consumer web --in-repo web --not-in-repo tool
seams list                      # the registered seams, one a line
seams show http:billing/invoices
seams validate                  # check the registry
seams detect --drift            # how the registry differs from the code
seams render                    # write docs/SEAMS.md
seams render --check            # in the gate: fail if docs/SEAMS.md is out of date
```

A seam's id is `<kind>:<provider>/<name>`. Run `seams` with no command for
every command, and `seams <command> -h` for its flags.

`--in-repo <party>` records that a party's code is in this repo, and
`--not-in-repo <party>` that it's in another, such as a tool the repo pins.
Each works on `seams add` and `seams update`, for the parties of the seam
it names. A party left unnamed has no record either way, and `seams` keeps
each party's record as it is, whatever else it changes. Tools read it as
`inRepo` in `seams list --json`: `true`, `false`, or left out when not
recorded.

`seams update --contract <location>` moves a seam's contract and keeps its
owner. Give `--owner` with it to name a new owner.

## For tools

Every command takes `--json`, and its output matches a published JSON
Schema in [`schemas/`](schemas). The schemas are the CLI's contract: a
schema breaks its readers only when its major version rises. Each release
carries them. Sheepdog uses `seams` only through this output.

## Docs

- [Detection](docs/DETECTION.md): how `seams detect` finds candidates, and
  how far to trust them.
- [The register](docs/REGISTER.md): `seams show` and `render`, and the
  coverage ladder.
- [Test policy](docs/TEST-POLICY.md): which tests run where.
- [Contributing](CONTRIBUTING.md): setting up, the gate, and releasing.

## Licence

Apache License 2.0: see [LICENSE](LICENSE).
