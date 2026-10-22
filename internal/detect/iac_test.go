package detect

import (
	"slices"
	"testing"
)

func TestTerraformEnvNamesReachQueuesAndOtherComponents(t *testing.T) {
	cands := detectFiles(t, map[string]string{
		"web/package.json":     `{"name": "web"}`,
		"web/index.js":         "fetch(process.env.PRICING_URL);\nsqs.send(new SendMessageCommand({ QueueUrl: process.env.JOBS }));\n",
		"worker/package.json":  `{"name": "worker"}`,
		"pricing/package.json": `{"name": "pricing"}`,
		"infra/main.tf": `
resource "aws_sqs_queue" "jobs" {}

resource "aws_lambda_function" "pricing" {
  function_name = "pricing"
}

resource "aws_lambda_function" "web" {
  environment {
    variables = {
      PRICING_URL = aws_lambda_function.pricing.function_url
    }
  }
}

resource "aws_ecs_task_definition" "worker" {
  family = "worker"
  container_definitions = jsonencode([{
    environment = [{ name = "JOBS", value = aws_sqs_queue.jobs.url }]
  }])
}

resource "aws_lambda_function" "unknown" {
  environment {
    variables = {
      JOBS = aws_sqs_queue.jobs.url
    }
  }
}

resource "aws_lambda_event_source_mapping" "jobs" {
  event_source_arn = aws_sqs_queue.jobs.arn
  function_name    = "elsewhere"
}
`,
	})
	pricing := mustFind(t, cands, "http:pricing/api", "pricing", "web")
	if pricing.Confidence != "high" {
		t.Errorf("an env name Terraform sets to another component gives %s", pricing.Confidence)
	}
	// web's JOBS is set nowhere for web, but only worker's environment sets it.
	jobs := mustFind(t, cands, "queue:infra/jobs", "infra", "web")
	if !slices.Contains(signals(jobs), sigEnv) {
		t.Errorf("signals %v", signals(jobs))
	}
}
