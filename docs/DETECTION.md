# Detection

`seams detect` proposes the seams a repo has, for a person to confirm. It
reads the repo's files as text: manifests, code, contracts and
infrastructure as code. It has no parser for any language and no cgo, so it
runs in a second or two and cross-compiles. That makes it good enough to
propose, not to decide. Each candidate comes with its evidence, by file and
line, and a confidence.

```sh
seams detect                 # the candidates, with their evidence
seams detect --json          # the same, to schemas/detect.json
seams detect --drift         # how the registry differs from the code; exits 1 if it does
seams detect --root ../shop  # another repo, with the registry in its .sheepdog/
```

## Components

A component is a part of the repo that can take part in a seam. A
directory with a manifest is one, named by the manifest: the last element
of `go.mod`'s module (without a `/vN` suffix), `package.json`'s name
(without its scope), `pyproject.toml`'s name, or `serverless.yml`'s
service. A file outside every such directory belongs to its top-level
directory, named after it, or to the repo, named after the root.

## Candidates and their ids

A candidate's id is derived as the registry's are,
`<kind>:<provider>/<name>`, so it matches a registered seam, a rejection,
or a fixture's true seam by id.

- When the evidence doesn't name an API, such as an HTTP server framework,
  its name is `api`: a provider's one API of that kind. Where the provider
  also has exactly one named API of the kind, such as one Backstage names,
  the two merge.
- When the evidence doesn't say who provides a seam, its provider is `_`,
  and its notes say so. A topic or queue that several parties publish to, or
  receive from as its owner, has no single owner, so it's `_` too, and its
  notes name each.
- A party that only uses its own API isn't a seam. Nor is a table only its
  owner reads.

## Confidence

Each piece of evidence has a confidence: `low`, `medium` or `high`. A
candidate's is its strongest evidence's, one higher when two or more
signals agree, and at most `medium` when nobody is known to provide it.

## Signals

Each piece of evidence names its signal, and its party's role: `provides`;
`consumes`; `owns`, which provides unless other evidence names a provider;
or `supports`, which makes a seam likelier without naming it.

| Signal | What it reads | Role |
|---|---|---|
| `catalog` | Backstage `catalog-info.yaml`: `providesApis` and `consumesApis`, and each API's type | provides, consumes |
| `pact` | A Pact file's consumer and provider; a message pact's are an event | provides, consumes |
| `pact-library` | Pact's library in a manifest | supports |
| `contract` | An OpenAPI or AsyncAPI document, a `.proto`'s services, a GraphQL schema | provides; owns a service or channel, which may sit in a shared place |
| `generator` | A client generator's config (Orval, openapi-generator, Kiota) and the spec it reads; `buf.gen.yaml` | consumes; supports |
| `compat-check` | A CI step that checks a contract's compatibility (oasdiff, openapi-diff, `buf breaking`, graphql-inspector, optic), or Pact's `can-i-deploy` | provides; supports |
| `http-server` | An HTTP or GraphQL server framework in a manifest, or a standard library server | provides |
| `http-route` | A route the code serves | provides |
| `http-client` | A call to a service by its host name, such as `http://pricing:8080` | consumes |
| `route-call` | A call to a path that exactly one other component serves | consumes |
| `grpc` | A gRPC server's registration, or a stub | provides, consumes |
| `org-client` | A dependency on the org's own `*-client` or `*-sdk` package | consumes |
| `workspace` | A dependency on another of the repo's packages | consumes |
| `vendor` | A vendor's SDK in a manifest or imported, or a call to its API's host | consumes |
| `broker` | A message broker's client in a manifest | supports |
| `schema-registry` | A schema registry's client in a manifest | supports |
| `messaging` | A producer's or consumer's call naming its topic or queue | provides, consumes |
| `env` | An env name ending `_URL`, `_TOPIC`, `_QUEUE` and the like, followed through IaC's env blocks to what it's set to | provides, consumes |
| `iac-resource` | A topic, bus, stream or queue in Terraform or CloudFormation | owns |
| `iac-gateway` | An API gateway or ingress, which makes what it fronts a public provider | provides |
| `iac-subscription` | A subscription or event-source mapping, which makes the function it delivers to a receiver | provides, consumes |
| `remote-state` | `terraform_remote_state`, or `Fn::ImportValue`: a seam to another stack, often another repo's | consumes |
| `serverless` | A Serverless or SAM function's events | provides, consumes |
| `migrations` | A table a component's migrations create | provides |
| `db-client` | A database client's component naming a table another component owns | consumes |
| `config-schema` | A config file with a `$schema`, other than a tool's own | consumes |

### Messages

An event's publisher provides it, and its subscribers consume it. A
queue's receiver provides it, and its senders consume it. Which way a
message goes is read from the nearest line that receives or sends, so a
named topic with neither nearby is `low`.

### Env names

An env name's ending says what it names: `_QUEUE_URL`, `_QUEUE` and the
like a queue; `_TOPIC_ARN`, `_TOPIC`, `_EVENT_BUS` and the like an event;
`_GRAPHQL_URL` a GraphQL API; `_URL` an HTTP API. Its stem names the
seam, so `PRICING_URL` is pricing's API. Where Terraform, CloudFormation,
SAM or Serverless sets the name in the component's environment, detection
follows it there instead, to the queue, topic, other component or URL it's
set to. Two components reading different names set to one queue then share
one seam. Names of databases, brokers, telemetry and the app itself, such as
`DATABASE_URL`, aren't seams.

### Third parties

A vendor's seam is `http:<vendor>/api`, with the vendor as an external
provider. Where its SDK is imported in more than one module (directory),
the candidate's notes say so, naming each site: with no SDK of the repo's
own around the vendor, each module depends on its API directly.

## Ecosystems

| | Go | TypeScript and JavaScript | Python |
|---|---|---|---|
| Manifests | `go.mod` | `package.json` | `pyproject.toml` (PEP 621 and Poetry), `requirements.txt` |
| HTTP servers | gin, echo, chi, fiber, `net/http` | Express, Fastify, Koa, NestJS, Hono, `http` | FastAPI, Flask, Django |
| GraphQL servers | gqlgen, graphql-go | Apollo Server, GraphQL Yoga | Strawberry, Graphene, Ariadne |
| gRPC | grpc-go | @grpc/grpc-js | grpcio |
| Brokers | kafka-go, Sarama, NATS, amqp091-go, AWS SQS, SNS and EventBridge | KafkaJS, amqplib, NATS, AWS SQS, SNS and EventBridge | confluent-kafka, kafka-python, pika, nats-py, boto3 |
| Databases | pgx, pq, GORM, sqlx, MySQL | pg, postgres, mysql2, Knex, Prisma, TypeORM, Sequelize | psycopg, SQLAlchemy, asyncpg |

Vendors: Stripe, Twilio, SendGrid, OpenAI, Anthropic, Slack, GitHub,
Mailgun, Postmark, Algolia and Plaid. Infrastructure as code: Terraform,
CloudFormation, SAM and Serverless. The JVM and Spring aren't read yet.

`internal/detect/libs.go` holds each list.

## What isn't read

Vendored code, virtual environments, build output, tests and QA tooling
hold no seams of the repo's own, so these are left out:

- `vendor/`, `node_modules/`, `bower_components/`, `third_party/`;
- `venv/`, any directory holding `pyvenv.cfg`, `__pycache__/`, `site-packages/`;
- `dist/`, `build/`, `out/`, `target/`, `bin/`, `coverage/`, `cdk.out/`;
- `test/`, `tests/`, `__tests__/`, `e2e/`, `cypress/`, `playwright/`, `fixtures/`, `testdata/`;
- hidden directories, other than `.github/workflows/`, which say what CI checks;
- test files (`*_test.go`, `*.test.*`, `*.spec.*`, `test_*.py`, `*_test.py`,
  `conftest.py`), generated code (`*.pb.go`, `*_pb2.py`, `*_pb2_grpc.py`),
  minified code, binary files and files over 1 MiB.

## Rejections and drift

A candidate the team rejects with `seams reject`, naming its id and the
signal and file of one piece of its evidence, isn't proposed again.

`seams detect --drift` lists the candidates the registry lacks, and the
registered seams no evidence is left for, each with why, and exits 1 when
there are any. A candidate matches a registered seam by its id, or one the
seam had. Where the evidence couldn't name part of it, it matches by the
rest: a provider's `api` matches its one registered API of the kind, and a
seam with provider `_` matches by kind and name. A registered seam of a kind
detection has no signal for, such as `cli` or `file`, is never stale.

## Fixtures

Each signal has a fixture repo under `testdata/signals/`, and
`testdata/system/` is a small polyglot system whose true seams are listed
in `testdata/system.seams.json`. Detection must find at least 80% of them.
Each fixture is built for the purpose and public; a real project's code is
never used.
