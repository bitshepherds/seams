package detect

import (
	"slices"
	"strings"
	"testing"
)

const cloudFormation = `
AWSTemplateFormatVersion: "2010-09-09"
Resources:
  Jobs:
    Type: AWS::SQS::Queue
  Alerts:
    Type: AWS::SNS::Topic
    Properties:
      TopicName: alerts
  Worker:
    Type: AWS::Lambda::Function
    Properties:
      Environment:
        Variables:
          JOBS_QUEUE_URL: !Ref Jobs
          BILLING_URL: https://billing.internal/
          STRIPE_URL: https://api.stripe.com
          GREETING: hello
  JobsMapping:
    Type: AWS::Lambda::EventSourceMapping
    Properties:
      EventSourceArn: !GetAtt Jobs.Arn
      FunctionName: !Ref Worker
  AlertsSubscription:
    Type: AWS::SNS::Subscription
    Properties:
      TopicArn: {Ref: Alerts}
      Endpoint: {"Fn::GetAtt": [Worker, Arn]}
  ElsewhereSubscription:
    Type: AWS::SNS::Subscription
    Properties:
      TopicArn: arn:aws:sns:eu-west-2:123456789012:elsewhere
  Api:
    Type: AWS::ApiGateway::RestApi
  Untyped:
    Properties: {}
Outputs:
  JobsArn:
    Value: !GetAtt Jobs.Arn
    Export:
      Name: shared-jobs
`

const worker = `
const { SQSClient, ReceiveMessageCommand } = require('@aws-sdk/client-sqs');

const poll = () => new SQSClient({}).send(new ReceiveMessageCommand({ QueueUrl: process.env.JOBS_QUEUE_URL }));
const bill = () => fetch(process.env.BILLING_URL);
const pay = () => fetch(process.env.STRIPE_URL);
`

func TestCloudFormationTemplateDeliversItsQueueAndTopicToFunctions(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"template.yaml":       cloudFormation,
		"worker/package.json": `{"name": "worker"}`,
		"worker/index.js":     worker,
	})
	jobs := mustFind(t, cands, "queue:worker/jobs", "worker")
	if !slices.Contains(signals(jobs), sigIaCSubscription) || !slices.Contains(signals(jobs), sigEnv) {
		t.Errorf("queue:worker/jobs signals %v", signals(jobs))
	}
	mustFind(t, cands, "event:repo/alerts", "repo", "worker")
	if api := mustFind(t, cands, "http:repo/api", "repo"); api.Audience != "public" {
		t.Errorf("the gateway's API is %s", api.Audience)
	}
	mustFind(t, cands, "http:billing/api", "billing", "worker")
	mustFind(t, cands, "http:stripe/api", "stripe", "worker")
	if len(cands) != 5 {
		t.Errorf("candidates %v", ids(cands))
	}
}

func TestSAMFunctionServesAnAPIAndIgnoresEventsItCannotFollow(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"pricing/package.json": `{"name": "pricing"}`,
		"template.yaml": `
Transform: AWS::Serverless-2016-10-31
Resources:
  Pricing:
    Type: AWS::Serverless::Function
    Properties:
      CodeUri: nowhere/
      Events:
        Get:
          Type: Api
        Nightly:
          Type: Schedule
        Missing:
          Type: SQS
          Properties:
            Queue: !GetAtt Absent.Arn
`,
	})
	if c := mustFind(t, cands, "http:pricing/api", "pricing"); c.Audience != "public" || len(cands) != 1 {
		t.Errorf("%+v; candidates %v", c, ids(cands))
	}
}

func TestStacksImportValuesExportedHereOrElsewhere(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"template.yaml": cloudFormation,
		"importer/template.yaml": `
AWSTemplateFormatVersion: "2010-09-09"
Resources:
  Bucket:
    Type: AWS::S3::Bucket
    Properties:
      BucketName: !ImportValue shared-jobs
      Tags:
        Fn::ImportValue: network-vpc
      Prefix:
        Fn::ImportValue: !Sub "${Env}-prefix"
`,
		"broken/template.yaml": "AWS::\n: [\n",
	})
	mustFind(t, cands, "config:repo/shared-jobs", "repo", "importer")
	vpc := mustFind(t, cands, "config:_/network-vpc", "", "importer")
	if len(vpc.Notes) != 1 || !strings.Contains(vpc.Notes[0], "another repo") {
		t.Errorf("notes %q", vpc.Notes)
	}
}

func TestServerlessEventsEachReceiveFromTheirSource(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"serverless.yml": `
service: alerts
functions:
  quiet:
    events: {}
  busy:
    events:
      - http:
          path: alerts
      - schedule: rate(1 hour)
      - sns:
          topicName: alerts
      - sqs: arn:aws:sqs:eu-west-2:123456789012:jobs
      - stream:
          arn:
            Fn::GetAtt: [Feed, Arn]
      - eventBridge:
          eventBus: ${self:custom.bus}
resources:
  Resources:
    Feed:
      Type: AWS::Kinesis::Stream
      Properties:
        Name: feed
`,
	})
	mustFind(t, cands, "http:alerts/api", "alerts")
	mustFind(t, cands, "event:_/alerts", "", "alerts")
	mustFind(t, cands, "queue:alerts/jobs", "alerts")
	mustFind(t, cands, "event:alerts/feed", "alerts")
	if len(cands) != 4 {
		t.Errorf("candidates %v", ids(cands))
	}
}
