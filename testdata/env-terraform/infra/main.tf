resource "aws_sqs_queue" "orders" {
  name = "orders"
}

resource "aws_lambda_function" "checkout" {
  function_name = "checkout"
  filename      = "../checkout/dist.zip"
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  role          = aws_iam_role.lambda.arn

  environment {
    variables = {
      ORDERS_URL = aws_sqs_queue.orders.url
    }
  }
}

resource "aws_lambda_function" "fulfilment" {
  function_name = "fulfilment"
  filename      = "../fulfilment/dist.zip"
  handler       = "poll.handler"
  runtime       = "nodejs20.x"
  role          = aws_iam_role.lambda.arn

  environment {
    variables = {
      INBOX_URL = aws_sqs_queue.orders.url
    }
  }
}
