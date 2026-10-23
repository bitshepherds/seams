# The register

The register is every seam in the registry, with its characteristics and
test world, so the team sees all of them in one place, not only the ones it
happens to know. `seams render` commits it as `docs/SEAMS.md`, and the gate
checks that it's up to date.

seams keeps its own files in `.sheepdog/seams/`, and writes nowhere else
under `.sheepdog/`, which other tools share: the registry, `registry.json`;
the register's history, `history.json`; and the status file it was last
given, `status.json`.

```sh
seams show                        # the register, one seam after another
seams show --json                 # the same, to schemas/register.json
seams show --status status.json   # each seam's rung too, gaps first
seams render                      # write docs/SEAMS.md
seams render --record             # record today's count in the history too
seams render --check              # write nothing; exit 1 if docs/SEAMS.md is out of date
```

## What each seam shows

Its id and kind, its provider and consumers, marked when they're external
or in this repo, and its audience. Where its contract lives and who owns
it, whether its sides deploy independently, and its test world: each
contract test, its side, the tier it runs in, and the tool that runs it
(`--tool` on `seams add` or `update`, which names the tool for the `--test`
contract tests given with it).

A seam crosses out of the system when any of its parties is external, or
its audience is public. `docs/SEAMS.md` lists those apart from the internal
seams, under a Mermaid diagram of the components and the seams between
them. A seam with no single owner is drawn as a node of its own.

## The coverage ladder

How well a seam is covered is judged by another tool, such as sheepdog,
never by seams. That tool writes a status file to `schemas/status.json`,
giving each seam's rung and its gaps, and passes it in with `--status`. The
rungs are:

| Rung | The seam has reached |
| --- | --- |
| 0 | nothing yet |
| 1 | the seam, its parties and its audience are known |
| 2 | a versioned, linted contract |
| 3 | each change checked for compatibility against trunk and the last release |
| 4 | consumer tests against the contract |
| 5 | the provider verified against it |
| 6 | a deploy gate, where the sides deploy separately |

A rating is found by the seam's id, or by an id it had before a rename. One
for an id the registry doesn't hold is named on stderr, not dropped
silently. With a status file, the gaps come first: the seams it doesn't
rate, then the lowest rung up. Without one, the register shows no rungs.

`seams render --status` keeps the file it was given in
`.sheepdog/seams/status.json`, and reads it when given none. So
`seams render --check` in the gate draws the same register that was
committed, and a later render keeps the rungs until a new status file
replaces them.

## History

`seams render --record` adds today's count, in UTC, to
`.sheepdog/seams/history.json` (`schemas/history.json`): how many seams
there are, and with a status file how many at each rung. A second record on
the same day replaces the first. `docs/SEAMS.md` shows the history, so the
team sees whether coverage keeps pace as seams grow.
