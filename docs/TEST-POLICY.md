# Test policy

How this repo's tests are split into tiers, so the gate stays fast and every
test still runs somewhere. `seams` is a Go CLI. It calls no service, model or
database, and deploys nothing: it's published as binaries. Its public API is
its JSON output, and the registry file, each described by a JSON Schema.

## The tiers

| Tier | When it runs | Budget | Command |
| --- | --- | --- | --- |
| Pre-commit | before every commit | seconds | `mise run fast` |
| The gate | before every push, and in CI on each pull request and each merge to `main` | under 10 minutes; about 6 seconds today | `mise run check` |
| Scheduled | weekly, in CI | 30 minutes | the gate, then fuzzing |
| Release | on each tag `vX.Y.Z` | | the gate, then GoReleaser |

Nothing is too slow for the gate, so there's no separate CI-only tier and no
after-merge suite. A test that ever needs one is moved by changing this
policy first.

## Which tier each kind of test runs in

- **Unit tests run in the gate.** All of `seams` is deterministic code: id
  parsing, the registry, detection over files, rendering. Each is tested with
  fast `go test` unit tests, runnable locally with no network. Detection is
  tested against fixture repos under `testdata/`, built for the purpose and
  public. A real project's code is never used as a fixture.
- **Scenarios run in the gate.** Behaviour seen from outside, each `seams`
  command and what it prints, is specified as Gherkin scenarios in
  `features/`, in the terms of the developers and tools that use it. gobdd
  runs them through `go test`, against a real build of `seams`, driven as a
  subprocess.
- **The API is checked in the gate.** Every JSON document `seams` writes, its
  output and the registry file, is validated against its published schema, in
  the tests that produce it. A compatibility test compares each schema with
  the one in the last release tag, read from git, and fails on a break the
  schema's version doesn't declare.
- **Fuzz tests run in two tiers.** Each parser of outside input, such as seam
  ids and the registry file, has a Go fuzz test. Its seed corpus runs in the
  gate, as an ordinary test. The weekly scheduled run fuzzes each target for a
  few minutes, and a failure files an issue with the crashing input, which
  then joins the seed corpus.
- **Release checks run on a tag.** GoReleaser builds every platform there;
  the gate only checks its config. Each release candidate is run before its
  final tag, outside this repo or through `mise exec`, as `CONTRIBUTING.md`
  says, since inside it the pin in `mise.toml` runs the pinned release.

## Coverage

Unit coverage is 100% of statements, checked in the gate by go-test-coverage.
Code that can't sensibly be unit tested, such as `main`'s one line that calls
`os.Exit`, is marked `// coverage-ignore` with its reason on the same line.
The scenarios cover it from outside. An omission without a reason is a bug.

## Flaky tests

A flaky test is a failing test. It's never retried silently. It's quarantined
with `t.Skip`, naming an issue that has an owner and a deadline, and fixed or
deleted by then.

## The tools

| What | Tool |
| --- | --- |
| Runner | `go test` |
| Coverage | go-test-coverage, from `.testcoverage.yml` |
| Scenarios | gobdd, not godog, whose MPL-2.0 dependencies the licence check refuses |
| Schemas | `santhosh-tekuri/jsonschema/v6` |
| Fuzzing | Go's own (`go test -fuzz`) |

Mutation testing isn't used yet. It's worth adding on a schedule once the
detector has enough logic for weak tests to hide in.

<!-- sheepdog test-strategy: {"written":"2026-10-22","project":{"languages":["Go"],"kinds":["cli"],"callsAi":false,"thirdParties":[],"datastores":[],"infrastructure":[],"slowDependencies":[],"exposesApi":"public","deploys":"never","stage":"prototype"},"tiers":{"unit":"gate","scenarios":"gate","api":"gate","fuzz-seeds":"gate","fuzzing":"scheduled","release":"release"}} -->
