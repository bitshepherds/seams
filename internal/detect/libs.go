package detect

// The libraries each signal looks for in manifests, by ecosystem: Go module
// paths, npm names and PyPI names. docs/DETECTION.md lists them too.

// httpServers are the server frameworks that make a component an HTTP
// provider.
var httpServers = []string{
	"github.com/gin-gonic/gin", "github.com/labstack/echo", "github.com/go-chi/chi", "github.com/gofiber/fiber",
	"express", "fastify", "koa", "@nestjs/core", "hono",
	"fastapi", "flask", "django",
}

// graphqlServers make a component a GraphQL provider.
var graphqlServers = []string{
	"github.com/99designs/gqlgen", "github.com/graph-gophers/graphql-go",
	"@apollo/server", "apollo-server", "graphql-yoga",
	"strawberry-graphql", "graphene", "ariadne",
}

// grpcLibs let a component's code be read for gRPC servers and stubs.
var grpcLibs = []string{"google.golang.org/grpc", "@grpc/grpc-js", "grpcio"}

// brokers are message brokers' clients: a component with one is read for
// producer and consumer calls.
var brokers = []string{
	"github.com/segmentio/kafka-go", "github.com/IBM/sarama", "github.com/Shopify/sarama",
	"github.com/nats-io/nats.go", "github.com/rabbitmq/amqp091-go",
	"github.com/aws/aws-sdk-go-v2/service/sqs", "github.com/aws/aws-sdk-go-v2/service/sns",
	"github.com/aws/aws-sdk-go-v2/service/eventbridge",
	"kafkajs", "amqplib", "nats", "@aws-sdk/client-sqs", "@aws-sdk/client-sns", "@aws-sdk/client-eventbridge",
	"confluent-kafka", "kafka-python", "pika", "nats-py", "boto3",
}

// schemaRegistries are schema-registry clients, which make a component's
// events likelier seams with a schema.
var schemaRegistries = []string{
	"github.com/riferrei/srclient", "@kafkajs/confluent-schema-registry", "python-schema-registry-client",
}

// pactLibs are Pact's libraries: a component with one tests a seam's
// contract.
var pactLibs = []string{"github.com/pact-foundation/pact-go", "@pact-foundation/pact", "pact-python"}

// dbClients are database clients: a component with one is read for the
// tables it reads.
var dbClients = []string{
	"github.com/jackc/pgx", "github.com/lib/pq", "gorm.io/gorm", "github.com/jmoiron/sqlx",
	"github.com/go-sql-driver/mysql",
	"pg", "postgres", "mysql2", "knex", "@prisma/client", "typeorm", "sequelize",
	"psycopg", "psycopg2", "psycopg2-binary", "sqlalchemy", "asyncpg",
}

// vendor is a third party a repo calls: its SDKs, the modules its code
// imports them as, and its API's hosts.
type vendor struct {
	name      string
	deps      []string // Go modules, npm and PyPI names
	pyModules []string
	hosts     []string
}

var vendors = []vendor{
	{"stripe", []string{"github.com/stripe/stripe-go", "stripe"}, []string{"stripe"}, []string{"api.stripe.com"}},
	{"twilio", []string{"github.com/twilio/twilio-go", "twilio"}, []string{"twilio"}, []string{"api.twilio.com"}},
	{"sendgrid", []string{"github.com/sendgrid/sendgrid-go", "@sendgrid/mail", "sendgrid"}, []string{"sendgrid"}, []string{"api.sendgrid.com"}},
	{"openai", []string{"github.com/openai/openai-go", "openai"}, []string{"openai"}, []string{"api.openai.com"}},
	{"anthropic", []string{"github.com/anthropics/anthropic-sdk-go", "@anthropic-ai/sdk", "anthropic"}, []string{"anthropic"}, []string{"api.anthropic.com"}},
	{"slack", []string{"github.com/slack-go/slack", "@slack/web-api", "slack-sdk"}, []string{"slack_sdk"}, []string{"slack.com/api", "hooks.slack.com"}},
	{"github", []string{"github.com/google/go-github", "@octokit/rest", "pygithub"}, []string{"github"}, []string{"api.github.com"}},
	{"mailgun", []string{"github.com/mailgun/mailgun-go", "mailgun.js"}, nil, []string{"api.mailgun.net"}},
	{"postmark", []string{"postmark", "postmarker"}, []string{"postmarker"}, []string{"api.postmarkapp.com"}},
	{"algolia", []string{"github.com/algolia/algoliasearch-client-go", "algoliasearch"}, []string{"algoliasearch"}, []string{"algolia.net"}},
	{"plaid", []string{"github.com/plaid/plaid-go", "plaid", "plaid-python"}, []string{"plaid"}, []string{"plaid.com"}},
}

// notServices are stems of env names ending _URL that name a database, a
// broker, telemetry or the app itself, rather than a service it calls.
var notServices = map[string]bool{
	"database": true, "db": true, "redis": true, "mongo": true, "mongodb": true, "postgres": true,
	"postgresql": true, "mysql": true, "amqp": true, "rabbitmq": true, "kafka": true, "broker": true,
	"nats": true, "sentry": true, "otel": true, "otel-exporter-otlp": true, "otlp": true, "jaeger": true,
	"elasticsearch": true, "s3": true, "aws": true, "cdn": true, "public": true, "site": true, "app": true,
	"base": true, "frontend": true, "callback": true, "redirect": true, "webhook": true, "cors": true,
	"origin": true, "localstack": true, "aws-endpoint": true, "queue": true, "topic": true,
	// API_URL, in a front end, names its back end, but not which component that is.
	"api": true, "backend": true, "server": true,
}

// toolingSchemas are hosts whose schemas describe tools' own config files,
// such as an editor's, rather than a seam of the system's.
var toolingSchemas = []string{
	"json-schema.org", "schemastore.org", "json.schemastore.org", "docs.renovatebot.com", "biomejs.dev",
	"unpkg.com", "raw.githubusercontent.com/SchemaStore", "turbo.build", "turborepo.com", "openapi.vercel.sh",
	"vercel.com", "ui.shadcn.com", "deno.land", "jsr.io", "nx.dev", "railway.com", "fly.io",
}
