data "terraform_remote_state" "network" {
  backend = "s3"
  config = {
    bucket = "acme-terraform"
    key    = "network.tfstate"
  }
}

resource "aws_sqs_queue" "emails" {
  name = "emails"
}

resource "aws_lambda_function" "checkout" {
  function_name = "checkout"
  filename      = "../services/checkout/dist.zip"
  handler       = "index.handler"
  runtime       = "nodejs20.x"
  role          = aws_iam_role.lambda.arn

  environment {
    variables = {
      EMAILS_QUEUE_URL = aws_sqs_queue.emails.url
      PRICING_URL      = "http://pricing.internal:8080"
    }
  }
}

resource "aws_lambda_function" "mailer" {
  function_name = "mailer"
  filename      = "../services/mailer/dist.zip"
  handler       = "handler.send"
  runtime       = "nodejs20.x"
  role          = aws_iam_role.lambda.arn
}

resource "aws_lambda_event_source_mapping" "emails" {
  event_source_arn = aws_sqs_queue.emails.arn
  function_name    = aws_lambda_function.mailer.arn
}

resource "aws_apigatewayv2_api" "shop" {
  name          = "shop"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_integration" "checkout" {
  api_id           = aws_apigatewayv2_api.shop.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.checkout.invoke_arn
}
